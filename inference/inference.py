from vllm import LLM, SamplingParams
import redis
import os
import json
import gc
import sys


redisHost = os.getenv('REDIS_ADDRESS') or 'err no host'
redisPassword = os.getenv('REDIS_PASSWORD') or 'err no pw'
redisPort = os.getenv('REDIS_PORT') or 'err no port'
r = redis.Redis(host=redisHost, port=int(redisPort), password=redisPassword, decode_responses=True)
print("started")
params=None

def process_batch(model, batch_prompts, batch_task_ids):
    global params
    # get the inference params in here to reduce risk of drift
    params = json.loads(r.get("inference") or 'ERROR')
    sampling_params = SamplingParams(
        max_tokens=params["max_new_tokens"],
        n=params["num_return_sequences"],
        best_of=params["num_beams"],
        temperature=0.0,
        spaces_between_special_tokens=False,
        use_beam_search=True,
    )
    generated = model.generate(batch_prompts, sampling_params)
    return generated

def send_results(generated, batch_prompts, batch_task_ids):
    global params
    num_sequences_per_prompt = (params or {})["num_return_sequences"]
    print("num_sequences_per_prompt", num_sequences_per_prompt)
    for i in range(len(batch_prompts)):
        model_outputs = []
        for j in range(num_sequences_per_prompt):
            model_output = generated[i].outputs[j].text
            # TODO: This is likely bad. Need to fix this 
            model_output = model_output.strip()  # Remove leading and trailing whitespace if any
            prompt = batch_prompts[i]
            print("=" * 5 + "prompt "+str(i))
            print(prompt)
            print("-" * 5 + "output "+str(i))
            print(model_output)
            model_outputs.append(model_output)
        result = {'task_id': batch_task_ids[i], 'result': model_outputs}
        result_string = json.dumps(result)
        r.lpush("results", result_string)

def main():
    # https://github.com/vllm-project/vllm/blob/bc96d5c330e079fa501eee05e97bf15009c9a094/vllm/entrypoints/llm.py#L24
    # model = LLM(model="../../models/Mistral-7B-v0.1", max_model_len=512)
    model = LLM(model=sys.argv[1], max_model_len=512, gpu_memory_utilization=0.85)
    # TODO: try disable_sliding_window
    # engine_args = EngineArgs(model=sys.argv[1], max_model_len=512, swap_space=16, gpu_memory_utilization=0.85)
    # engine = LLMEngine.from_engine_args(engine_args)

    batch_size = 32
    batch_prompts = []
    batch_task_ids = []

    while True:
        print("=" * 40 + "Starting batch building")
        while len(batch_prompts) < batch_size:
            message = r.brpoplpush("tasks","processing", timeout=5)  # timeout of 5 seconds
            if message:
                msg = message
                json_msg = json.loads(msg)
                task_id = json_msg["task_id"]
                input = json_msg["task"]
                instruction = json_msg["instruction"]
                prompt= f"[INST]\n{instruction}:\n{input}\n[/INST]"
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
