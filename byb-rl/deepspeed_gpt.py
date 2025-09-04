# %%%
import numpy as np
import pandas as pd
import os
import ray
from datasets import load_dataset
model_name = "EleutherAI/gpt-j-6B"
use_gpu = True
num_workers = 12
cpus_per_worker = 8

# ray.init()

# %%
print("Loading tiny_shakespeare dataset")
current_dataset = load_dataset("tiny_shakespeare" )
current_dataset
# %%
import ray.data

ray_datasets = {
    "train": ray.data.from_huggingface(current_dataset["train"]),
    "validation": ray.data.from_huggingface(current_dataset["validation"]),
}

ray_datasets
# %%
block_size = 512
from transformers import AutoTokenizer


def split_text(batch: pd.DataFrame) -> pd.DataFrame:
    text = list(batch["text"])
    flat_text = "".join(text)
    split_text = [
        x.strip()
        for x in flat_text.split("\n")
        if x.strip() and not x.strip()[-1] == ":"
    ]
    return pd.DataFrame(split_text, columns=["text"])


def tokenize(batch: pd.DataFrame) -> dict:
    tokenizer = AutoTokenizer.from_pretrained(model_name, use_fast=False)
    tokenizer.pad_token = tokenizer.eos_token
    ret = tokenizer(
        list(batch["text"]),
        truncation=True,
        max_length=block_size,
        padding="max_length",
        return_tensors="np",
    )
    ret["labels"] = ret["input_ids"].copy()
    return dict(ret)


processed_datasets = {
    key: (
        ds.map_batches(split_text, batch_format="pandas")
        .map_batches(tokenize, batch_format="pandas")
    )
    for key, ds in ray_datasets.items()
}
processed_datasets

# %%
import evaluate
import torch
from transformers import (
    Trainer,
    TrainingArguments,
    GPTJForCausalLM,
    AutoTokenizer,
    default_data_collator,
)
from transformers.utils.logging import disable_progress_bar, enable_progress_bar

from ray import train
from ray.train.huggingface.transformers import prepare_trainer, RayTrainReportCallback


def train_func(config):
    import sys
    import time
    
    print("Starting train_func...")
    sys.stdout.flush()
    
    # Get Ray Train context safely
    try:
        context = train.get_context()
        rank = context.get_world_rank()
        world_size = context.get_world_size()
        local_rank = context.get_local_rank()
        print(f"[Rank {rank}/{world_size}] Ray context initialized - local_rank={local_rank}")
    except Exception as e:
        print(f"Error getting Ray context: {e}")
        rank = 0
        world_size = 1
        local_rank = 0
    
    sys.stdout.flush()
    
    # Use the actual number of CPUs assigned by Ray
    try:
        cpu_count = train.get_context().get_trial_resources().bundles[-1].get("CPU", 1)
        os.environ["OMP_NUM_THREADS"] = str(cpu_count)
        print(f"[Rank {rank}] Set OMP_NUM_THREADS to {cpu_count}")
    except Exception as e:
        print(f"[Rank {rank}] Error setting OMP_NUM_THREADS: {e}")
        os.environ["OMP_NUM_THREADS"] = "1"
    
    # Enable tf32 for better performance
    torch.backends.cuda.matmul.allow_tf32 = True
    
    print(f"[Rank {rank}] Environment setup complete")
    sys.stdout.flush()

    batch_size = config.get("batch_size", 4)
    epochs = config.get("epochs", 2)
    warmup_steps = config.get("warmup_steps", 0)
    learning_rate = config.get("learning_rate", 0.00002)
    weight_decay = config.get("weight_decay", 0.01)
    steps_per_epoch = config.get("steps_per_epoch")

    deepspeed = {
        "fp16": {
            "enabled": "auto",
            "initial_scale_power": 8,
            "hysteresis": 4,
            "consecutive_hysteresis": True,
        },
        "bf16": {"enabled": "auto"},
        "optimizer": {
            "type": "AdamW",
            "params": {
                "lr": "auto",
                "betas": "auto",
                "eps": "auto",
            },
        },
        "zero_optimization": {
            "stage": 2,
            "offload_optimizer": {
                "device": "none",
            },
            "overlap_comm": True,
            "contiguous_gradients": True,
            "reduce_bucket_size": "auto",
            "stage3_prefetch_bucket_size": "auto",
            "stage3_param_persistence_threshold": "auto",
            "gather_16bit_weights_on_model_save": True,
            "round_robin_gradients": True,
        },
        "gradient_accumulation_steps": "auto",
        "gradient_clipping": "auto",
        "steps_per_print": 10,
        "train_batch_size": "auto",
        "train_micro_batch_size_per_gpu": "auto",
        "wall_clock_breakdown": False,
    }

    print("Preparing training arguments")
    training_args = TrainingArguments(
        "output",
        logging_steps=1,
        save_strategy="steps",
        save_steps=steps_per_epoch,
        max_steps=steps_per_epoch * epochs,
        per_device_train_batch_size=batch_size,
        gradient_accumulation_steps=1,
        learning_rate=learning_rate,
        weight_decay=weight_decay,
        warmup_steps=warmup_steps,
        label_names=["input_ids", "attention_mask"],
        push_to_hub=False,
        report_to="none",
        disable_tqdm=True,  # declutter the output a little
        bf16=True,
        gradient_checkpointing=True,
        deepspeed=deepspeed,
    )
    disable_progress_bar()

    tokenizer = AutoTokenizer.from_pretrained(model_name)
    tokenizer.pad_token = tokenizer.eos_token

    print(f"[Rank {rank}] Loading model: {model_name}")
    sys.stdout.flush()

    model = GPTJForCausalLM.from_pretrained(model_name, use_cache=False)
    model.resize_token_embeddings(len(tokenizer))

    print(f"[Rank {rank}] Model loaded, resized embeddings to {len(tokenizer)}")
    sys.stdout.flush()

    enable_progress_bar()

    metric = evaluate.load("accuracy")

    print(f"[Rank {rank}] Getting dataset shards...")
    sys.stdout.flush()
    
    train_ds = train.get_dataset_shard("train")
    eval_ds = train.get_dataset_shard("validation")

    print(f"[Rank {rank}] Creating dataset iterables...")
    sys.stdout.flush()
    
    train_ds_iterable = train_ds.iter_torch_batches(
        batch_size=batch_size,
        local_shuffle_buffer_size=train.get_context().get_world_size() * batch_size,
    )
    eval_ds_iterable = eval_ds.iter_torch_batches(batch_size=batch_size)
    
    print(f"[Rank {rank}] Dataset iterables created")
    sys.stdout.flush()

    def compute_metrics(eval_pred):
        logits, labels = eval_pred
        predictions = np.argmax(logits, axis=-1)
        return metric.compute(predictions=predictions, references=labels)

    trainer = Trainer(
        model=model,
        args=training_args,
        train_dataset=train_ds_iterable,
        eval_dataset=eval_ds_iterable,
        compute_metrics=compute_metrics,
        tokenizer=tokenizer,
        data_collator=default_data_collator,
    )

    # Add callback to report checkpoints to Ray Train
    trainer.add_callback(RayTrainReportCallback())
    trainer = prepare_trainer(trainer)
    trainer.train()
# %%
storage_path = "/data/zaporter/ray"
batch_size = 64
train_ds_size = processed_datasets["train"].count()
steps_per_epoch = train_ds_size // (batch_size * num_workers)
from ray.train.torch import TorchTrainer
from ray.train import RunConfig, ScalingConfig

trainer = TorchTrainer(
    train_loop_per_worker=train_func,
    train_loop_config={
        "epochs": 1,
        "batch_size": batch_size,  # per device
        "steps_per_epoch": steps_per_epoch,
    },
    scaling_config=ScalingConfig(
        num_workers=num_workers,
        use_gpu=use_gpu,
        resources_per_worker={"GPU": 1, "CPU": cpus_per_worker},
    ),
    datasets=processed_datasets,
    run_config=RunConfig(storage_path=storage_path),
)

# %%
results = trainer.fit()
# %%
