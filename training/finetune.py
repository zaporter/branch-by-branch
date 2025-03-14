import gc
import json
import time
from typing import Any, Tuple, Union, Optional
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
from datetime import datetime
from zoneinfo import ZoneInfo
from datasets import load_dataset
import random

parser = argparse.ArgumentParser()
parser.add_argument("--learning_rate", type=float, default=6e-5)
parser.add_argument("--batch_size", type=int, default=4)
parser.add_argument("--epochs", type=int, default=1)
parser.add_argument("--autogroup_tokens", type=int, default=1024)
parser.add_argument("--model", type=str, required=True)
parser.add_argument("--adapter", type=str, required=True)
parser.add_argument("--new_adapter_name", type=str, required=True)
parser.add_argument("--train_data", type=str, required=True)
args = parser.parse_args()

redisHost = os.getenv('REDIS_ADDRESS') or 'err no host'
redisPassword = os.getenv('REDIS_PASSWORD') or 'err no pw'
redisPort = os.getenv('REDIS_PORT') or 'err no port'
r = redis.Redis(host=redisHost, port=int(redisPort), password=redisPassword, decode_responses=True)

print("started")


# Dict[TrainingGroupID, TrainingDataGroup]
all_data = {}

advertisement_index: int = 0



def empty_to_none(s:str) -> str | None:
    return None if s == "" else s

def local_model_dir(name:str):
    return f"{os.getenv('HOME')}/cache/models/{name}/base"

def local_adapter_dir(name:str, adapter_name:str):
    return f"{os.getenv('HOME')}/cache/models/{name}/{adapter_name}"

def local_adapter_json(name:str, adapter_name:str):
    return local_adapter_dir(name, adapter_name)+".json"

def new_adapter_name() -> str:
    currentTime = datetime.now(ZoneInfo("America/New_York"))
    return f"adapter-{currentTime.strftime('%Y-%m-%dT%H:%M:%S')}"

def download_model(model_name:str):
    print("downloading model", model_name)
    rclone_cmd = f"../scripts/rclone-model.sh {model_name}"
    print("running rclone", rclone_cmd)
    out = os.system(rclone_cmd)
    if out != 0:
        raise Exception(f"Failed to download model {model_name}")
    print("downloaded model", model_name)


def load_trainer():
    print("loading model")
    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_compute_dtype=torch.float16
    )
    model = AutoModelForCausalLM.from_pretrained(
        pretrained_model_name_or_path=local_model_dir(args.model),
        trust_remote_code=True,
        low_cpu_mem_usage=True,
        quantization_config=bnb_config,
        max_position_embeddings=8192,
        device_map="auto",
        use_cache=False  # Disable KV cache during training
    )
    
    # Enable gradient checkpointing for memory efficiency
    model.gradient_checkpointing_enable()
    
    tokenizer = AutoTokenizer.from_pretrained(local_model_dir(args.model), padding_side="left")
    tokenizer.pad_token_id = tokenizer.eos_token_id
    print("loaded model preparing for lora")
    target_modules = ["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"]

    lora_config = LoraConfig(
        r=64,
        lora_alpha=64,
        target_modules=target_modules,
        task_type="CAUSAL_LM",
    )

    # Prepare model for k-bit training
    for param in model.parameters():
        param.requires_grad = False  # Freeze the entire model

    model = PeftModel.from_pretrained(
        model=model, 
        model_id=local_adapter_dir(args.model, args.adapter),
        config=lora_config,
        max_position_embeddings=8192,
        is_trainable=True,
    )
    
    # Enable gradient checkpointing after PEFT model creation
    if hasattr(model, "enable_input_require_grads"):
        model.enable_input_require_grads()
    
    model.print_trainable_parameters()
    
    # Move model to device
    if torch.cuda.device_count() > 1:
        print(f"Using {torch.cuda.device_count()} GPUs!")
        #model = torch.nn.DataParallel(model)
    #model = model.cuda()
    
    trainer = Trainer(model, tokenizer)
    return trainer

class Trainer:
    def __init__(self, model, tokenizer):
        self.model = model
        self.tokenizer = tokenizer
        
        self.device = next(model.parameters()).device
        self.optimizer = AdamW(self.model.parameters(), lr=args.learning_rate)
        # Initialize loss function once
        self.loss_fct = nn.CrossEntropyLoss(reduction='none')
    
    def _prepare_batch(self, batch):
        """
        Prepare a batch of data for training.
        
        Returns a dictionary containing tokenized prompts and completions.
        """
        prompt_texts = []
        completion_texts = []
        
        for item in batch:
            # Handle potential missing fields
            prompt = item.get("prompt", "")
            completion = item.get("completion", "")
            
            if not isinstance(prompt, str):
                prompt = str(prompt)
            if not isinstance(completion, str):
                completion = str(completion)
            
            prompt_texts.append(prompt)
            completion_texts.append(completion)
        
        # Tokenize prompts
        prompt_encodings = self.tokenizer(
            prompt_texts, 
            padding=True, 
            return_tensors="pt",
        ).to(self.device)
        
        # Tokenize completions
        completion_encodings = self.tokenizer(
            completion_texts, 
            padding=True, 
            return_tensors="pt",
        ).to(self.device)
        
        return {
            "prompt_ids": prompt_encodings["input_ids"],
            "prompt_mask": prompt_encodings["attention_mask"],
            "completion_ids": completion_encodings["input_ids"],
            "completion_mask": completion_encodings["attention_mask"]
        }
    
    def train_step(self, batch):
        """
        Perform a single training step.
        """
        self.model.train()
        inputs = self._prepare_batch(batch)
        
        prompt_ids = inputs["prompt_ids"]
        prompt_mask = inputs["prompt_mask"]
        completion_ids = inputs["completion_ids"]
        completion_mask = inputs["completion_mask"]
        
        # Concatenate for full sequence processing
        input_ids = torch.cat([prompt_ids, completion_ids], dim=1)
        attention_mask = torch.cat([prompt_mask, completion_mask], dim=1)
        
        # Forward pass
        outputs = self.model(
            input_ids=input_ids,
            attention_mask=attention_mask,
            return_dict=True
        )
        
        logits = outputs.logits
        
        # Shift for language modeling (predict next token)
        shift_logits = logits[..., :-1, :].contiguous()
        shift_labels = input_ids[..., 1:].contiguous()
        
        # Create completion mask that matches the shifted tensors
        # First, create a mask that's 1 for completion tokens and 0 for prompt tokens
        full_completion_mask = torch.cat([
            torch.zeros_like(prompt_mask),
            torch.ones_like(completion_mask)
        ], dim=1)
        # Then shift it to match the shifted logits/labels
        shift_completion_mask = full_completion_mask[..., :-1].contiguous()
        
        # Compute loss
        # Flatten the tensors
        flat_shift_logits = shift_logits.view(-1, shift_logits.size(-1))
        flat_shift_labels = shift_labels.view(-1)
        flat_shift_mask = shift_completion_mask.view(-1)
        
        # Compute per-token losses
        losses = self.loss_fct(flat_shift_logits, flat_shift_labels)
        
        # Apply completion mask and compute mean loss
        masked_losses = losses * flat_shift_mask
        loss = masked_losses.sum() / (flat_shift_mask.sum() + 1e-8)
        
        # Backward pass
        loss.backward()
        
        return loss.item()
    
    def train_step_microbatch(self, batch):
        """
        Handle microbatching to prevent OOM errors.
        """
        total_loss = 0
        num_microbatches = max(1, len(batch) // args.batch_size)
        microbatch_size = max(1, len(batch) // num_microbatches)
        
        for i in range(0, len(batch), microbatch_size):
            microbatch = batch[i:i + microbatch_size]
            loss = self.train_step(microbatch)
            total_loss += loss
            
            # Clean up memory
            gc.collect()
            torch.cuda.empty_cache()
        
        return total_loss / num_microbatches
    
    def train(self, dataset):
        """Train the model on the given dataset."""
        global all_data
        self.model.train()
        
        # Prepare dataset
        def prepare_example(example):
            # Ensure we have both prompt and completion fields
            if isinstance(example, str):
                return {"prompt": "", "completion": example}
            if isinstance(example, dict):
                if "prompt" not in example or "completion" not in example:
                    # Try to handle different field names
                    prompt = example.get("prompt", example.get("input", example.get("source", "")))
                    completion = example.get("completion", example.get("output", example.get("target", example.get("text", ""))))
                    return {"prompt": prompt, "completion": completion}
            return example
        
        dataset = dataset.map(prepare_example)
        
        # Collect training data
        all_data = {"training_samples": len(dataset["train"])}
        print(f"Training on {len(dataset['train'])} examples")
        
        # Training loop
        for epoch in range(args.epochs):
            print(f"Starting epoch {epoch+1}/{args.epochs}")
            epoch_loss = 0
            num_batches = 0
            
            # Create batches
            train_data = dataset["train"]
            indices = list(range(len(train_data)))
            random.shuffle(indices)
            
            # Process batches with progress bar
            progress_bar = tqdm(range(0, len(indices), args.batch_size))
            for batch_start in progress_bar:
                batch_end = min(batch_start + args.batch_size, len(indices))
                batch_indices = indices[batch_start:batch_end]
                batch = [train_data[i] for i in batch_indices]
                
                # Train on batch
                batch_loss = self.train_step_microbatch(batch)
                epoch_loss += batch_loss
                num_batches += 1
                
                # Update model parameters
                self.optimizer.step()
                self.optimizer.zero_grad()
                
                # Update progress bar
                progress_bar.set_description(f"Loss: {batch_loss:.4f}")
                
                # Periodically clean memory
                if num_batches % 10 == 0:
                    gc.collect()
                    torch.cuda.empty_cache()
            
            # End of epoch
            avg_epoch_loss = epoch_loss / num_batches
            print(f"Epoch {epoch+1} finished. Average loss: {avg_epoch_loss:.4f}")
            all_data[f"epoch_{epoch+1}_loss"] = avg_epoch_loss
        
        # Save the fine-tuned model
        adapter_name = self.save_and_upload_model()
        return adapter_name
    
    def save_and_upload_model(self):
        adapter_name = args.new_adapter_name
        with open(local_adapter_json(args.model, adapter_name), "w+") as data_file:
            # Save training stats
            dataToSave = json.dumps(all_data)
            data_file.write(dataToSave)
            
        self.model.save_pretrained(local_adapter_dir(args.model, adapter_name))
        self.upload_model(adapter_name)
        print(f"Model {adapter_name} saved")
        return adapter_name
    
    def upload_model(self, adapter_name: str):
        rclone_cmd = f"../scripts/rclone-push.sh {args.model}/{adapter_name} {local_adapter_dir(args.model, adapter_name)}"
        out = os.system(rclone_cmd)
        if out != 0:
            raise Exception(f"Failed to upload model {adapter_name}")


def main():
    download_model(args.model+"/base")
    download_model(args.model+"/"+args.adapter)

    if not os.path.exists(local_model_dir(args.model)):
        print("base model dir does not exist")
        print("you must manually cache it.")
        exit(1)

    if not os.path.exists(local_adapter_dir(args.model, args.adapter)):
        print("adapter model dir does not exist")
        print("you must manually cache it.")
        exit(1)

    dataset = load_dataset("json", data_files=args.train_data)
    print("Dataset structure:", dataset)
    print("First example:", dataset["train"][0])
    
    trainer = load_trainer()
    adapter_name = trainer.train(dataset)
    print(f"Training complete. New adapter: {adapter_name}")


if __name__ == "__main__":
    main()
