from typing import Any, Union, Optional
from peft import LoraConfig, get_peft_model, PeftModel
from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig
import torch
import torch.nn as nn
from torch.optim import AdamW
import transformers
import redis
import os
import argparse
import trl
from tqdm import tqdm


parser = argparse.ArgumentParser()
parser.add_argument("--pissa_load_and_save", type=bool, required=False, default=False)
parser.add_argument("--pissa_quantize_res", type=bool, required=False, default=False)
parser.add_argument("--new_model_name", type=str, required=False, default=None)
parser.add_argument("--learning_rate", type=float, default=1e-4)
parser.add_argument("--num_epochs", type=int, default=3)
parser.add_argument("--batch_size", type=int, default=4)
args = parser.parse_args()

redisHost = os.getenv('REDIS_ADDRESS') or 'err no host'
redisPassword = os.getenv('REDIS_PASSWORD') or 'err no pw'
redisPort = os.getenv('REDIS_PORT') or 'err no port'
r = redis.Redis(host=redisHost, port=int(redisPort), password=redisPassword, decode_responses=True)

print("started")

params=None

def empty_to_none(s:str) -> str | None:
    return None if s == "" else s

def local_model_dir(name:str):
    return f"{os.getenv('HOME')}/cache/models/{name}/base"

def local_adapter_dir(name:str, adapter_name:str):
    return f"{os.getenv('HOME')}/cache/models/{name}/{adapter_name}"

def download_model(model_name:str):
    rclone_cmd = f"../scripts/rclone-model.sh {model_name}"
    out = os.system(rclone_cmd)
    if out != 0:
        raise Exception(f"Failed to download model {model_name}")

def update_params():
    global params
    params = {
        "training_base_model": r.get("training:base_model"),
        "training_adapter": r.get("training:adapter"),
    }

def batch_generator():
    for i in range(50):
        yield {
            "prompt": f"Sup dude {i}\n",
            "outputs": [
                {
                    "output": f"Hello Bro {i}.",
                    "advantage": 1.0,
                },
                {
                    "output": f"Yo Brah {i}.",
                    "advantage": 0.0,
                }
            ]
        }

def load_trainer():
    print("loading model")
    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_compute_dtype=torch.bfloat16
    )
    model = AutoModelForCausalLM.from_pretrained(
        pretrained_model_name_or_path=local_model_dir(params["training_base_model"]),
        torch_dtype=torch.bfloat16,
        trust_remote_code=True,
        low_cpu_mem_usage=True,
        quantization_config=bnb_config
    )
    # load tokenizer to pass it through to the output dir
    tokenizer = AutoTokenizer.from_pretrained(local_model_dir(params["training_base_model"]), padding_side="left")
    tokenizer.pad_token_id = tokenizer.eos_token_id
    print("loaded model preparing for lora")

    model = PeftModel.from_pretrained(
        model=model, 
        model_id=local_adapter_dir(params["training_base_model"], params["training_adapter"]),
        is_trainable=True,
    )
    # https://huggingface.co/docs/peft/en/quicktour
    model.print_trainable_parameters()
    trainer = Trainer(model, tokenizer)
    return trainer

class Trainer:
    def __init__(self, model, tokenizer):
        self.model = model
        self.tokenizer = tokenizer
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.model.to(self.device)
        self.optimizer = AdamW(self.model.parameters(), lr=args.learning_rate)
        self.beta = 0.1  # KL penalty coefficient
        
    def _get_logprobs(self, logits, labels):
        """Compute per-token log probabilities."""
        # Get log probabilities for all tokens
        log_probs = torch.log_softmax(logits, dim=-1)
        
        # Get the log probabilities for the actual tokens
        token_log_probs = log_probs.gather(
            dim=-1, 
            index=labels.unsqueeze(-1)
        ).squeeze(-1)
        
        return token_log_probs
        
    def _prepare_batch(self, batch):
        # Prepare prompts and responses with advantages
        prompts = []
        responses = []
        group_indices = []  # Track which items belong to same prompt group
        
        for group_idx, item in enumerate(batch):
            prompt = item["prompt"]
            for output in item["outputs"]:
                prompts.append(prompt)
                responses.append(output["output"])
                group_indices.append(group_idx)
        
        # Tokenize inputs
        inputs = self.tokenizer(
            prompts,
            text_target=responses,
            padding=True,
            truncation=True,
            return_tensors="pt",
            max_length=512
        )
        
        # Move everything to device
        inputs = {k: v.to(self.device) for k, v in inputs.items()}
        group_indices = torch.tensor(group_indices, device=self.device)
        
        return inputs, group_indices

    def train_step(self, batch):
        self.model.train()
        inputs, group_indices = self._prepare_batch(batch)
        
        # Get outputs from current model (policy)
        policy_outputs = self.model(**inputs)
        policy_logits = policy_outputs.logits
        
        # Get per-token log probabilities for policy
        policy_log_probs = self._get_logprobs(policy_logits, inputs["labels"])
        
        # Get outputs from reference model (no gradients needed)
        with torch.no_grad():
            with self.model.disable_adapter():  # Use base model as reference
                ref_outputs = self.model(**inputs)
                ref_logits = ref_outputs.logits
                ref_log_probs = self._get_logprobs(ref_logits, inputs["labels"])
        
        # Create attention mask for completion tokens
        completion_mask = (inputs["labels"] != self.tokenizer.pad_token_id).float()
        
        # Compute KL divergence term per token
        per_token_kl = torch.exp(ref_log_probs - policy_log_probs) - (ref_log_probs - policy_log_probs) - 1
        
        # Compute advantages per group
        unique_groups = torch.unique(group_indices)
        total_loss = torch.tensor(0.0, device=self.device)
        
        for group in unique_groups:
            group_mask = (group_indices == group)
            group_policy_logprobs = policy_log_probs[group_mask]
            group_completion_mask = completion_mask[group_mask]
            group_kl = per_token_kl[group_mask]
            
            # Compute sequence-level scores for advantage calculation
            seq_scores = (group_policy_logprobs * group_completion_mask).sum(dim=1) / group_completion_mask.sum(dim=1)
            advantages = (seq_scores - seq_scores.mean()).detach()
            
            # Compute per-token policy gradient loss with advantages
            per_token_loss = torch.exp(group_policy_logprobs - group_policy_logprobs.detach()) * advantages.unsqueeze(1)
            per_token_loss = -(per_token_loss - self.beta * group_kl)
            
            # Apply completion mask and average
            group_loss = ((per_token_loss * group_completion_mask).sum(dim=1) / group_completion_mask.sum(dim=1)).mean()
            total_loss += group_loss
            
        total_loss = total_loss / len(unique_groups)
        
        # Backward pass
        self.optimizer.zero_grad()
        total_loss.backward()
        self.optimizer.step()
        
        return total_loss.item()

    def train(self, data_generator, num_epochs=None):
        if num_epochs is None:
            num_epochs = args.num_epochs
            
        for epoch in range(num_epochs):
            total_loss = 0
            num_batches = 0
            
            # Create progress bar for the epoch
            pbar = tqdm(data_generator(), desc=f"Epoch {epoch+1}/{num_epochs}")
            
            # Initialize batch list at start of epoch
            batch = []
            
            for item in pbar:
                batch.append(item)
                
                if len(batch) >= args.batch_size:
                    # Process the batch
                    loss = self.train_step(batch)
                    total_loss += loss
                    num_batches += 1
                    pbar.set_postfix({'loss': loss})
                    
                    # Reset batch after processing
                    batch = []
                    torch.cuda.empty_cache()  # Clear GPU memory
            
            # Handle remaining items in batch at end of epoch
            if batch:
                loss = self.train_step(batch)
                total_loss += loss
                num_batches += 1
                batch = []  # Clear the batch
                torch.cuda.empty_cache()
            
            # Print epoch summary
            avg_loss = total_loss/num_batches if num_batches > 0 else float('inf')
            print(f"Epoch {epoch+1}/{num_epochs}, Average Loss: {avg_loss:.4f}")
            
            # Clear memory between epochs
            torch.cuda.empty_cache()
    
    def save_model(self):
        save_dir = local_adapter_dir(params["training_base_model"], params["training_adapter"])+"bar"
        self.model.save_pretrained(save_dir)
        print(f"Model saved to {save_dir}")


def main():
    update_params()
    download_model(params["training_base_model"])

    if not os.path.exists(local_model_dir(params["training_base_model"])):
        print("model dir does not exist")
        print("you must manually cache it.")
        exit(1)

    trainer = load_trainer()
    trainer.train(batch_generator)
    trainer.save_model()


if __name__ == "__main__":
    main()
