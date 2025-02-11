from peft import LoraConfig, get_peft_model
from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig
import torch
import transformers
import redis
import os
import argparse


parser = argparse.ArgumentParser()
parser.add_argument("--pissa_load_and_save", type=bool, required=False, default=False)
parser.add_argument("--pissa_quantize_res", type=bool, required=False, default=False)
parser.add_argument("--new_model_name", type=str, required=False, default=None)
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
    return f"{os.getenv('HOME')}/cache/models/{name}"

def download_model(model_name:str):
    rclone_cmd = f"../scripts/rclone-model.sh {model_name}"
    out = os.system(rclone_cmd)
    if out != 0:
        raise Exception(f"Failed to download model {model_name}")

def update_params():
    global params
    params = {
        "training_base_model": r.get("training:base_model"),
        "model_dir": r.get("inference:model_dir"),
        "adapter_dir": r.get("inference:adapter_dir"),
        "load_format": empty_to_none(r.get("inference:load_format")), # ex: bitsandbytes or ""
        "batch_size": int(r.get("inference:batch_size")),
        "max_model_len": int(r.get("inference:max_model_len")),
        "gpu_memory_utilization": float(r.get("inference:gpu_memory_utilization")),
        "max_new_tokens": int(r.get("inference:max_new_tokens")),
        "num_return_sequences": int(r.get("inference:num_return_sequences")),
        "num_beams": int(r.get("inference:num_beams")),
    }



def batch_generator():
    for i in range(10):
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

def pissa_load_and_save():
    print("loading model")
    model = AutoModelForCausalLM.from_pretrained(
        pretrained_model_name_or_path=local_model_dir(params["training_base_model"]),
        # Do I need to do something to support unsloth / quantization?
        torch_dtype=torch.bfloat16,
        low_cpu_mem_usage=True,
        trust_remote_code=True
    )
    # load tokenizer to pass it through to the output dir
    tokenizer = AutoTokenizer.from_pretrained(local_model_dir(params["training_base_model"]))
    print("loaded model preparing for lora")

    lora_config = LoraConfig(
        # https://huggingface.co/docs/peft/en/developer_guides/lora
        init_lora_weights="pissa_niter_4",
        r=32,
        lora_alpha=32,
        target_modules=["q_proj", "o_proj", "k_proj", "v_proj", "gate_proj", "up_proj", "down_proj"],
        lora_dropout=0.0,
        bias="none",
        task_type="CAUSAL_LM"
    )
    # https://huggingface.co/docs/peft/en/quicktour
    model = get_peft_model(model, lora_config)
    model.print_trainable_parameters()
    print("saving model")

    outputDir = local_model_dir(args.new_model_name)
    os.makedirs(outputDir, exist_ok=True)

    model.save_pretrained(f"{outputDir}/pissa_init")
    # unloads peft model, leaving Wres.. I think
    model = model.unload()
    model.save_pretrained(outputDir)
    tokenizer.save_pretrained(outputDir)

def main():
    update_params()

    download_model(params["training_base_model"])

    if not os.path.exists(local_model_dir(params["training_base_model"])):
        print("model dir does not exist")
        print("you must manually cache it.")
        exit(1)

    if args.pissa_load_and_save:
        pissa_load_and_save()


# TODO:
# https://github.com/huggingface/trl/blob/55e680e142d88e090dcbf5a469eab1ebba28ddef/trl/trainer/grpo_trainer.py#L625
def compute_loss():
    pass

if __name__ == "__main__":
    main()
