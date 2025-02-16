import json
import time
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

redis_training_recv_chan = "training:data-chan"
redis_training_req_chan = "training:request-chan"
redis_training_adv_list = "training:advertisement-list"

params=None

# Dict[TrainingGroupID, TrainingDataGroup]
all_data = {}

advertisement_index: int = 0

def get_next_advertisement():
    global advertisement_index
    next_key = r.lindex(redis_training_adv_list, advertisement_index)
    return next_key



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
    global advertisement_index
    global all_data
    outstanding_requests = 0
    while True:
        data = r.rpop(redis_training_recv_chan)
        if data is None:
            if outstanding_requests >= 8:
                time.sleep(1)
                continue
            next_adv = get_next_advertisement()
            if next_adv is None:
                print("no more advertisements. Training blocked.")
                time.sleep(1)
                continue
            advertisement_index += 1
            if all_data[next_adv] is not None:
                # already seen -- continue looping
                continue;
            r.lpush(redis_training_req_chan, next_adv)
            outstanding_requests += 1
            continue

        outstanding_requests -= 1
        data = json.loads(data)

        assert data["prompt"] is not None
        assert data["outputs"] is not None
        assert len(data["outputs"]) > 0
        assert data["outputs"][0]["output"] is not None
        assert data["outputs"][0]["advantage"] is not None

        all_data[data["group_id"]] = data

        yield data


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
        self.ref_model = None  # We'll use the same model as reference
        self.max_prompt_length = None  # No prompt length restriction by default
    
    def _get_per_token_logps(self, model, input_ids, attention_mask, logits_to_keep):
        # We add 1 to `logits_to_keep` because the last logits of the sequence is later excluded
        logits = model(input_ids=input_ids, attention_mask=attention_mask, logits_to_keep=logits_to_keep + 1).logits
        logits = logits[:, :-1, :]  # (B, L-1, V), exclude the last logit: it corresponds to the next token pred

        input_ids = input_ids[:, -logits_to_keep:]
        # For transformers<=4.48, logits_to_keep argument isn't supported, so here we drop logits ourselves.
        # See https://github.com/huggingface/trl/issues/2770
        logits = logits[:, -logits_to_keep:]
        return trl.trainer.utils.selective_log_softmax(logits, input_ids)  #  compute logprobs for the input tokens

    def _prepare_batch(self, batch):
        # Prepare prompts and responses with advantages
        prompts = []
        responses = []
        advantages = []
        group_indices = []  # Track which items belong to same prompt group
        
        for group_idx, item in enumerate(batch):
            prompt = item["prompt"]
            for output in item["outputs"]:
                prompts.append(prompt)
                responses.append(output["output"])
                advantages.append(output["advantage"])
                group_indices.append(group_idx)
        
        # Tokenize inputs
        inputs = self.tokenizer(
            prompts,
            padding=True,
            truncation=True,
            return_tensors="pt",
            max_length=512
        )
        
        # Tokenize responses separately
        response_tokens = self.tokenizer(
            responses,
            padding=True,
            truncation=True,
            return_tensors="pt",
            max_length=512
        )
        
        # Move everything to device
        inputs = {k: v.to(self.device) for k, v in inputs.items()}
        response_tokens = {k: v.to(self.device) for k, v in response_tokens.items()}
        advantages = torch.tensor(advantages, device=self.device)
        group_indices = torch.tensor(group_indices, device=self.device)
        
        return {
            "prompt_ids": inputs["input_ids"],
            "prompt_mask": inputs["attention_mask"],
            "completion_ids": response_tokens["input_ids"],
            "completion_mask": response_tokens["attention_mask"],
            "advantages": advantages,
            "group_indices": group_indices
        }

    def train_step(self, batch):
        self.model.train()
        inputs = self._prepare_batch(batch)
        
        prompt_ids = inputs["prompt_ids"]
        prompt_mask = inputs["prompt_mask"]
        completion_ids = inputs["completion_ids"]
        completion_mask = inputs["completion_mask"]
        advantages = inputs["advantages"]
        
        # Concatenate for full sequence processing
        input_ids = torch.cat([prompt_ids, completion_ids], dim=1)
        attention_mask = torch.cat([prompt_mask, completion_mask], dim=1)

        
        # Get reference model logprobs (using frozen reference model)
        with torch.inference_mode():
            ref_per_token_logps = self._get_per_token_logps(
                model=self.ref_model,
                input_ids=input_ids,
                attention_mask=attention_mask,
                logits_to_keep=completion_ids.size(1)
            )
        
        # Compute loss using the provided compute_loss function
        loss = self.compute_loss(self.model, {
            "prompt_ids": prompt_ids,
            "prompt_mask": prompt_mask,
            "completion_ids": completion_ids,
            "completion_mask": completion_mask,
            "ref_per_token_logps": ref_per_token_logps.detach(),  # Ensure reference logprobs are detached
            "advantages": advantages
        })
        
        # Backward pass
        self.optimizer.zero_grad()
        loss.backward()
        self.optimizer.step()
        
        return loss.item()

    def compute_loss(self, model, inputs):
        prompt_ids, prompt_mask = inputs["prompt_ids"], inputs["prompt_mask"]
        completion_ids, completion_mask = inputs["completion_ids"], inputs["completion_mask"]
        input_ids = torch.cat([prompt_ids, completion_ids], dim=1)
        attention_mask = torch.cat([prompt_mask, completion_mask], dim=1)
        logits_to_keep = completion_ids.size(1)  # we only need to compute the logits for the completion tokens

        per_token_logps = self._get_per_token_logps(model, input_ids, attention_mask, logits_to_keep)

        # Compute the KL divergence between the model and the reference model
        ref_per_token_logps = inputs["ref_per_token_logps"]
        per_token_kl = torch.exp(ref_per_token_logps - per_token_logps) - (ref_per_token_logps - per_token_logps) - 1

        # x - x.detach() allows for preserving gradients from x
        advantages = inputs["advantages"]
        per_token_loss = torch.exp(per_token_logps - per_token_logps.detach()) * advantages.unsqueeze(1)
        per_token_loss = -(per_token_loss - self.beta * per_token_kl)
        loss = ((per_token_loss * completion_mask).sum(dim=1) / completion_mask.sum(dim=1)).mean()

        return loss

    def train(self, data_generator, num_epochs=None):
        if num_epochs is None:
            num_epochs = args.num_epochs
            
        for epoch in range(num_epochs):
            self.ref_model = trl.models.modeling_base.create_reference_model(self.model)
            total_loss = 0
            num_batches = 0
            batch = []
            
            
            # Create progress bar for the epoch
            pbar = tqdm(data_generator(), desc=f"Epoch {epoch+1}/{num_epochs}")
            
            for item in pbar:
                batch.append(item)
                
                if len(batch) >= args.batch_size:
                    # Process the batch
                    loss = self.train_step(batch)
                    total_loss += loss
                    num_batches += 1
                    
                    # Update progress bar
                    avg_loss = total_loss / num_batches
                    pbar.set_postfix({'avg_loss': f'{avg_loss:.4f}'})
                    
                    # Reset batch
                    batch = []
                    torch.cuda.empty_cache()
            
            # Handle any remaining items in the last batch
            if batch:
                loss = self.train_step(batch)
                total_loss += loss
                num_batches += 1
            
            # Print epoch summary
            avg_loss = total_loss / num_batches if num_batches > 0 else float('inf')
            print(f"Epoch {epoch+1}/{num_epochs}, Average Loss: {avg_loss:.4f}")
            
            # Clean up reference model and clear memory between epochs
            torch.cuda.empty_cache()
        del self.ref_model
    
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
