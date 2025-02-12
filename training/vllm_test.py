# SPDX-License-Identifier: Apache-2.0
"""
This example shows how to use LoRA with different quantization techniques
for offline inference.

Requires HuggingFace credentials for access.
"""

import gc
import os
import sys
from typing import List, Optional, Tuple

import torch
from huggingface_hub import snapshot_download

from vllm import EngineArgs, LLMEngine, RequestOutput, SamplingParams
from vllm.lora.request import LoRARequest


def create_test_prompts(
        lora_path: Optional[str]
) -> List[Tuple[str, SamplingParams, Optional[LoRARequest]]]:
    return [
        # this is an example of using quantization without LoRA
        ("My name is",
         SamplingParams(temperature=0.0,
                        logprobs=1,
                        prompt_logprobs=1,
                        max_tokens=128), None),
        # the next three examples use quantization with LoRA
        ("my name is",
         SamplingParams(temperature=0.0,
                        logprobs=1,
                        prompt_logprobs=1,
                        max_tokens=128),
         LoRARequest("lora-test-1", 1, lora_path) if lora_path else None),
        ("The capital of USA is",
         SamplingParams(temperature=0.0,
                        logprobs=1,
                        prompt_logprobs=1,
                        max_tokens=128),
         LoRARequest("lora-test-2", 1, lora_path) if lora_path else None),
        ("The capital of France is",
         SamplingParams(temperature=0.0,
                        logprobs=1,
                        prompt_logprobs=1,
                        max_tokens=128),
         LoRARequest("lora-test-3", 1, lora_path) if lora_path else None),
    ]


def process_requests(engine: LLMEngine,
                     test_prompts: List[Tuple[str, SamplingParams,
                                              Optional[LoRARequest]]]):
    """Continuously process a list of prompts and handle the outputs."""
    request_id = 0

    while test_prompts or engine.has_unfinished_requests():
        if test_prompts:
            prompt, sampling_params, lora_request = test_prompts.pop(0)
            engine.add_request(str(request_id),
                               prompt,
                               sampling_params,
                               lora_request=lora_request)
            request_id += 1

        request_outputs: List[RequestOutput] = engine.step()
        for request_output in request_outputs:
            if request_output.finished:
                print("----------------------------------------------------")
                print(f"Prompt: {request_output.prompt}")
                print(f"Output: {request_output.outputs[0].text}")


def initialize_engine(model: str, quantization: Optional[str],
                      lora_path: Optional[str]) -> LLMEngine:
    """Initialize the LLMEngine."""

    if quantization == "bitsandbytes":
        # Load the pre-quantized model directly without re-quantization
        engine_args = EngineArgs(model=model,
                                quantization="bitsandbytes",  # Disable vLLM quantization
                                #qlora_adapter_name_or_path=lora_path,
                                load_format="bitsandbytes",  # Tell vLLM this is already in nf4 format
                                enable_lora=True,
                                max_lora_rank=64,
                                dtype="bfloat16")  # Use float16 for compute
    else:
        engine_args = EngineArgs(model=model,
                                 )
    return LLMEngine.from_engine_args(engine_args)



def main():
    base_path = sys.argv[1]
    adapter_path = sys.argv[2]
    """Main function that sets up and runs the prompt processing."""

    test_configs = [{
        "name": "qpissa",
        'model': f"{base_path}/base",
        'quantization': "bitsandbytes",
        'lora_path': f"{base_path}/{adapter_path}"
    },
    {
        "name": "baseline",
        'model': "/home/ubuntu/cache/models/meta/llama-3.1-8-instruct/base",
        'quantization': None,
        'lora_path': None
    }
    ]

    for test_config in test_configs:
        print(
            f"~~~~~~~~~~~~~~~~ Running: {test_config['name']} ~~~~~~~~~~~~~~~~"
        )
        engine = initialize_engine(test_config['model'],
                                   test_config['quantization'],
                                   test_config['lora_path'])
        test_prompts = create_test_prompts(test_config['lora_path'])
        process_requests(engine, test_prompts)

        # Clean up the GPU memory for the next test
        del engine
        gc.collect()
        torch.cuda.empty_cache()


if __name__ == '__main__':
    main()