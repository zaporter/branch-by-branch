from vllm import LLM, SamplingParams 
import redis
import os
import json
import gc
from vllm.sampling_params import GuidedDecodingParams
import re
import torch
# was crashing. This terrifies me.
import torch._dynamo
torch._dynamo.config.suppress_errors = True

redisHost = os.getenv('REDIS_ADDRESS') or 'err no host'
redisPassword = os.getenv('REDIS_PASSWORD') or 'err no pw'
redisPort = os.getenv('REDIS_PORT') or 'err no port'
r = redis.Redis(host=redisHost, port=int(redisPort), password=redisPassword, decode_responses=True)

print("started")
params=None
#pattern = r"<think>[^<]+</think><actions>.*</actions>"
# I wonder if the the any hurts performance.
grammar_str = r"""
root ::= "<think>" words "</think>\n<actions>" any "</actions>"
words ::= [^<]+
any ::= [^~]*
"""

def download_model(model_name:str):
    rclone_cmd = f"../scripts/rclone-model.sh {model_name}"
    out = os.system(rclone_cmd)
    if out != 0:
        raise Exception(f"Failed to download model {model_name}")

def update_params():
    global params
    params = {
        "enabled": r.get("inference:enabled") == "true",
        "model_dir": r.get("inference:model_dir"),
        "adapter_dir": r.get("inference:adapter_dir"),
        "load_format": r.get("inference:load_format"), # ex: bitsandbytes or ""
        "batch_size": int(r.get("inference:batch_size")),
        "max_model_len": int(r.get("inference:max_model_len")),
        "gpu_memory_utilization": float(r.get("inference:gpu_memory_utilization")),
        "max_new_tokens": int(r.get("inference:max_new_tokens")),
        "num_return_sequences": int(r.get("inference:num_return_sequences")),
        "num_beams": int(r.get("inference:num_beams")),
    }


def local_model_dir(name:str):
    return f"{os.getenv('HOME')}/cache/models/{name}"

def process_batch(model, batch_prompts, batch_task_ids):
    global params
    # get the inference params in here to reduce risk of drift
    update_params()
    guided_decoding_params = GuidedDecodingParams(grammar=grammar_str)
    sampling_params = SamplingParams(
        max_tokens=params["max_new_tokens"],
        n=params["num_return_sequences"],
        best_of=params["num_beams"],
        include_stop_str_in_output=True,
        guided_decoding=guided_decoding_params,
        temperature=0.1,
        top_p=0.9,
        stop=["</actions>"]
    )
    generated = model.generate(batch_prompts, sampling_params)
    return generated

def send_results(generated, batch_prompts, batch_task_ids):
    global params
    num_sequences_per_prompt = params["num_return_sequences"]
    print("num_sequences_per_prompt", num_sequences_per_prompt)
    for i in range(len(batch_prompts)):
        return_sequences = []
        for j in range(num_sequences_per_prompt):
            model_output = generated[i].outputs[j].text
            prompt = batch_prompts[i]
            #print("=" * 5 + "prompt "+str(i))
            #print(prompt)
            print("-" * 5 + "output "+str(i))
            print(model_output)
            return_sequences.append(model_output)

        inference_task_result = {
            "return_sequences": return_sequences,
        }
        result = {'task_id': batch_task_ids[i], 'result': json.dumps(inference_task_result)}
        result_string = json.dumps(result)
        r.lpush("inference-engine:results", result_string)

def main():
    update_params()
    if not params["enabled"]:
        print("inference is disabled")
        return

    download_model(params["model_dir"])

    if not os.path.exists(local_model_dir(params["model_dir"])):
        print("model dir does not exist")
        print("you must manually cache it.")
        exit(1)

    print("params", params)

    load_format = params["load_format"] if params["load_format"]!="" else None

    print("load_format", load_format)

    num_gpus = torch.cuda.device_count()

    print("num_gpus", num_gpus)
    # https://github.com/vllm-project/vllm/blob/bc96d5c330e079fa501eee05e97bf15009c9a094/vllm/entrypoints/llm.py#L24
    model = LLM(
        model=local_model_dir(params["model_dir"]),
        max_model_len=params["max_model_len"],
        gpu_memory_utilization=params["gpu_memory_utilization"],
        tensor_parallel_size=num_gpus,
        trust_remote_code=True,
        # same https://docs.vllm.ai/en/latest/features/quantization/bnb.html
        load_format=load_format,
        quantization=load_format,
        # https://docs.vllm.ai/en/stable/performance/optimization.html
        enable_chunked_prefill=True,
        # https://docs.vllm.ai/en/stable/performance/optimization.html
        max_num_batched_tokens=8192,
        # https://docs.vllm.ai/en/stable/features/automatic_prefix_caching.html
        enable_prefix_caching=True

    )
    # TODO: if the params for the LLM() constructor change, we need to reconstruct the model

    batch_size = params["batch_size"]
    batch_prompts = []
    batch_task_ids = []

    while True:
        print("=" * 40 + "Starting batch building")
        while len(batch_prompts) < batch_size:
            task = r.brpoplpush("inference-engine:tasks","inference-engine:processing", timeout=5)  # timeout of 5 seconds
            if task:
                # See orchestrator/inference.go & orchestrator/engine.go
                task_msg = json.loads(task)
                task_id = task_msg["task_id"]
                inference_task = json.loads(task_msg["task"])
                prompt = inference_task["prompt"]

                batch_prompts.append(prompt)
                batch_task_ids.append(task_id)
            else:
                # Timeout reached, process whatever we have if it's not empty
                if batch_prompts:
                    break

        if not batch_prompts:
            print("no prompts, should not be possible to reach here")
            continue  # No tasks, go back to waiting

        print("=" * 40 + "Starting batch. Len: " + str(len(batch_task_ids)))
        generated = process_batch(model, batch_prompts, batch_task_ids)

        send_results(generated, batch_prompts, batch_task_ids)
        del batch_prompts
        del batch_task_ids
        del generated

        batch_prompts=[]
        batch_task_ids=[]

        gc.collect()

if __name__ == "__main__":
    main()
