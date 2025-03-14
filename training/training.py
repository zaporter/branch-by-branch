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
import random

parser = argparse.ArgumentParser()
parser.add_argument("--pissa_load_and_save", type=bool, required=False, default=False)
parser.add_argument("--pissa_quantize_res", type=bool, required=False, default=False)
parser.add_argument("--new_model_name", type=str, required=False, default=None)
parser.add_argument("--learning_rate", type=float, default=6e-5)
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

def update_params():
    global params
    params = {
        "training_base_model": r.get("training:base_model"),
        "training_adapter": r.get("training:adapter"),
        "training_do_update_adapter": r.get("training:do_update_adapter") == "true",
        "training_autogroup_tokens": int(r.get("training:autogroup_tokens")),
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
            print("got advertisement", next_adv)
            advertisement_index += 1
            if next_adv in all_data:
                # already seen -- continue looping
                print("already seen", next_adv)
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

        #print("got data", data)

        yield data


def load_trainer():
    print("loading model")
    bnb_config = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_compute_dtype=torch.float16
    )
    model = AutoModelForCausalLM.from_pretrained(
        pretrained_model_name_or_path=local_model_dir(params["training_base_model"]),
        trust_remote_code=True,
        low_cpu_mem_usage=True,
        quantization_config=bnb_config,
        max_position_embeddings=8192,
        device_map="auto",
        use_cache=False  # Disable KV cache during training
    )
    
    # Enable gradient checkpointing for memory efficiency
    model.gradient_checkpointing_enable()
    
    tokenizer = AutoTokenizer.from_pretrained(local_model_dir(params["training_base_model"]), padding_side="left")
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
        model_id=local_adapter_dir(params["training_base_model"], params["training_adapter"]),
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
        
        # Set up device handling
        self.device = next(model.parameters()).device
        self.optimizer = AdamW(self.model.parameters(), lr=args.learning_rate)
        self.beta = 0.1  # KL penalty coefficient
        self.ref_model = None 
        self.max_prompt_length = None  # No prompt length restriction by default
    
    def _get_per_token_logps(self, model, input_ids, attention_mask, logits_to_keep):
        # Ensure inputs are on the same device as model
        input_ids = input_ids.to(self.device)
        attention_mask = attention_mask.to(self.device)
        
        # We add 1 to `logits_to_keep` because the last logits of the sequence is later excluded
        logits = model(input_ids=input_ids, attention_mask=attention_mask, logits_to_keep=logits_to_keep + 1).logits
        logits = logits[:, :-1, :]  # (B, L-1, V), exclude the last logit: it corresponds to the next token pred

        input_ids = input_ids[:, -logits_to_keep:]
        # For transformers<=4.48, logits_to_keep argument isn't supported, so here we drop logits ourselves.
        # See https://github.com/huggingface/trl/issues/2770
        logits = logits[:, -logits_to_keep:]
        return trl.trainer.utils.selective_log_softmax(logits, input_ids)

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
            truncation=False,
            return_tensors="pt",
        )
        
        # Tokenize responses separately
        response_tokens = self.tokenizer(
            responses,
            padding=True,
            truncation=False,
            return_tensors="pt",
        )
        
        # Move everything to the same device as the model
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

    def train_step_microbatch(self, batch, scale=1.0):
        total_loss = []
        groupScale = scale/len(batch)
        
        for group in batch:
            token_budget = params["training_autogroup_tokens"]
            # auto group by num tokens greedily filling up groups of token_budget tokens
            autoGroups = []
            nextGroup = []
            prompt_token_count = self.tokenizer(group["prompt"], add_special_tokens=False, return_length=True).length
            print("Z: prompt_token_count", prompt_token_count)
            # token_budget -= prompt_token_count[0]
            if token_budget < prompt_token_count[0]:
                raise Exception("token budget exceeded for group prompt "+group["prompt"])
            original_token_budget = token_budget

            for item in group["outputs"]:
                token_count = self.tokenizer(item["output"], add_special_tokens=False, return_length=True).length
                print("Z: token_count", token_count)
                # TODO: Does this belong here?
                token_budget -= prompt_token_count[0]
                token_budget -= token_count[0]
                if token_budget < 0:
                    autoGroups.append({"prompt": group["prompt"], "outputs": nextGroup})
                    nextGroup = []
                    token_budget = original_token_budget
                nextGroup.append(item)
            autoGroups.append({"prompt": group["prompt"], "outputs": nextGroup})
            print("Z: autoGroups", len(autoGroups))
            print("Z: autoGroups sublengths", [len(autogroup["outputs"]) for autogroup in autoGroups])
            print("Z: orggroups", len(group["outputs"]))

            for autoGroup in autoGroups:
                loss = self.train_step([autoGroup], scale=(len(autoGroup)*groupScale/len(group["outputs"])))
                total_loss += [loss]
                # Try to reduce vram usage.
                gc.collect()
                torch.cuda.empty_cache()
                
        return total_loss

    def train_step(self, batch, scale=1.0):
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
                #todo--does this correctly shift by the input size?
                logits_to_keep=completion_ids.size(1)
            )
        per_token_logps = self._get_per_token_logps(
            model=self.model, 
            input_ids=input_ids,
            attention_mask=attention_mask,
            logits_to_keep=completion_ids.size(1)
        )
        
        # Compute loss using the provided compute_loss function
        loss = self.compute_loss(
            per_token_logps=per_token_logps,
            ref_per_token_logps=ref_per_token_logps.detach(),
            advantages=advantages,
            completion_mask=completion_mask
        )
        loss = loss*scale
        loss.backward()

        return loss.item()
        
    def step(self):
        self.optimizer.step()
        self.optimizer.zero_grad()


    def compute_loss(self, per_token_logps, ref_per_token_logps, advantages, completion_mask):
        # Compute the KL divergence between the model and the reference model
        per_token_kl = torch.exp(ref_per_token_logps - per_token_logps) - (ref_per_token_logps - per_token_logps) - 1

        # x - x.detach() allows for preserving gradients from x
        per_token_loss = torch.exp(per_token_logps - per_token_logps.detach()) * advantages.unsqueeze(1)
        per_token_loss = -(per_token_loss - self.beta * per_token_kl)
        # global normalization https://github.com/huggingface/trl/pull/2881
        loss = (per_token_loss * completion_mask).sum() / completion_mask.sum()
        print("loss", loss.item())

        return loss

    def train(self, data_generator):
        global all_data
        #self.ref_model = trl.models.modeling_base.create_reference_model(self.model)
        self.ref_model = self.model
        total_loss = 0
        num_batches = 0
        batch = []
        
        
        # Create progress bar for the epoch
        pbar = tqdm(data_generator())
        
        for item in pbar:
            batch.append(item)
            update_params()
            
            if len(batch) >= args.batch_size:
                r.set("inference:enabled", "false")

                batchLosses = self.train_step_microbatch(batch, scale=2/3)
                historyBatch = random.sample(list(all_data.values()), k=args.batch_size)
                historyLosses = self.train_step_microbatch(historyBatch, scale=1/3)
                self.step()
                print("Z: loss items", batchLosses, historyLosses)
                total_loss += sum(batchLosses) + sum(historyLosses)
                num_batches += 1
                
                # Update progress bar
                avg_loss = total_loss / num_batches
                pbar.set_postfix({'avg_loss': f'{avg_loss:.4f}'})
                
                # Reset batch
                batch = []
                torch.cuda.empty_cache()
                
                adapter_name = self.save_and_upload_model()
                if params["training_do_update_adapter"]:
                    self.swap_adapter(adapter_name)
        
        # Handle any remaining items in the last batch
        if batch:
            print("Warn: Discarding extra data in batch due to generator termination")
        
        # Print epoch summary
        avg_loss = total_loss / num_batches if num_batches > 0 else float('inf')
        print(f"Epoch Average Loss: {avg_loss:.4f}")
        
        # Clean up reference model and clear memory between epochs
        torch.cuda.empty_cache()
        del self.ref_model
    
    def save_and_upload_model(self):
        adapter_name = new_adapter_name()
        with open(local_adapter_json(params["training_base_model"], adapter_name), "w+") as data_file:
            # TODO: probably better to save as jsonl. I am just testing.
            dataToSave = json.dumps(all_data)
            data_file.write(dataToSave)
            
        self.model.save_pretrained(local_adapter_dir(params["training_base_model"], adapter_name))
        self.upload_model(adapter_name)
        print(f"Model {adapter_name} saved")
        return adapter_name
    
    def upload_model(self, adapter_name: str):
        rclone_cmd = f"../scripts/rclone-push.sh {params['training_base_model']}/{adapter_name} {local_adapter_dir(params['training_base_model'], adapter_name)}"
        out = os.system(rclone_cmd)
        if out != 0:
            raise Exception(f"Failed to upload model {adapter_name}")

    def swap_adapter(self, adapter_name: str):
        r.set("inference:base_model", params["training_base_model"])
        r.set("inference:adapter", adapter_name)
        r.set("inference:enabled", "true")
        r.set("training:adapter", adapter_name)


def main():
    update_params()
    download_model(params["training_base_model"]+"/base")
    download_model(params["training_base_model"]+"/"+params["training_adapter"])

    if not os.path.exists(local_model_dir(params["training_base_model"])):
        print("base model dir does not exist")
        print("you must manually cache it.")
        exit(1)

    if not os.path.exists(local_adapter_dir(params["training_base_model"], params["training_adapter"])):
        print("adapter model dir does not exist")
        print("you must manually cache it.")
        exit(1)

    trainer = load_trainer()
    trainer.train(batch_generator)
    trainer.save_and_upload_model()


if __name__ == "__main__":
    main()
