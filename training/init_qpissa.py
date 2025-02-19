# THIS IS TAKEN FROM https://github.com/GraphPKU/PiSSA/blob/main/utils/init_qpissa.py
# The pissa_quant function is far better than what I can do with peft (because I can't figure out how to incorporate quant into the svd loop)
import torch
import argparse
from transformers import AutoModelForCausalLM, AutoTokenizer
from peft import get_peft_model, LoraConfig
import bitsandbytes as bnb
from tqdm import tqdm
import gc
# ./branch-by-branch/training/run_training.sh init_qpissa.py --base_model_dir ~/cache/models/meta/llama-3.1-8-instruct/base --output_dir ~/test --rank 64 --iter 2

parser = argparse.ArgumentParser(description="Initializing QPiSSA.")
parser.add_argument("--base_model_dir", type=str, required=True)
parser.add_argument("--output_dir", type=str, required=True)
parser.add_argument("--rank", type=int, default=64)
parser.add_argument("--iter", type=int, default=5)
parser.add_argument("--device", type=str, default="cpu")
args = parser.parse_args()

def quantize_and_dequantized(weight):
    device = weight.device
    weight_nf4 = bnb.nn.Params4bit(weight.to("cpu"), requires_grad=False, compress_statistics=False, quant_type="nf4")
    weight_nf4 = weight_nf4.to(device)
    weight_dequantized = bnb.functional.dequantize_4bit(
        weight_nf4.data, weight_nf4.quant_state
    ).to(torch.float32)
    return weight_nf4, weight_dequantized

@torch.no_grad()
def pissa_quant(weight, r=64, niter=5):
    # send to gpu to speed up svd but also because nf4 is only supported on gpu
    weight = weight.to("cuda")
    res = weight.to(torch.float32)
    for i in range(niter):
        U, S, Vh = torch.linalg.svd(res, full_matrices=False)
        L = U @ (torch.sqrt(torch.diag(S)[:, :r]))
        R = torch.sqrt(torch.diag(S)[:r, :]) @ Vh
        res = weight - L @ R
        weight_nf4, weight_dequantized = quantize_and_dequantized(res)
        res = weight - weight_dequantized

    return weight_nf4, weight_dequantized.to("cpu"), R.to("cpu"), L.to("cpu")

@torch.no_grad()
def convert_to_4bit_layer(module, weight_4bit):
    """Convert a linear layer to 4-bit with pre-computed weights"""
    new_layer = bnb.nn.Linear4bit(
        module.in_features,
        module.out_features,
        bias=module.bias is not None,
        compute_dtype=torch.float16,
        compress_statistics=False,
        quant_type="nf4"
    )
    # Set the pre-computed 4-bit weights
    new_layer.weight = weight_4bit
    if module.bias is not None:
        new_layer.bias = module.bias
    return new_layer

print("loading model")
base_model = AutoModelForCausalLM.from_pretrained(
    args.base_model_dir, torch_dtype=torch.bfloat16, device_map=args.device, low_cpu_mem_usage=True)
tokenizer = AutoTokenizer.from_pretrained(args.base_model_dir)
print("loaded model")

target_modules = ["q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj"]

lora_config = LoraConfig(
    r=args.rank,
    lora_alpha=args.rank,
    target_modules=target_modules,
    task_type="CAUSAL_LM",
)
peft_model = get_peft_model(base_model, peft_config=lora_config)


with torch.no_grad():
    print("Performing PISSA quantization and converting to 4-bit layers")
    for name, module in tqdm(peft_model.named_modules()):
        # Only process modules that are part of the transformer layers and match our target names
        if any(target in name for target in target_modules) and hasattr(module, 'base_layer'):
            print(f"Processing {name}")
            # Get original weight for PISSA quantization
            original_weight = module.base_layer.weight
            
            # Perform PISSA quantization on original weights
            base_layer_in_4bits, dequant, lora_A, lora_B = pissa_quant(original_weight, args.rank, args.iter)
            
            # Convert to 4-bit layer with pre-computed weights
            # This causes vllm to fail to load the model 😿
            # Need to save the dequant weights just for them to be requantized during loading.
            #new_layer = convert_to_4bit_layer(module.base_layer, base_layer_in_4bits)
            #module.base_layer = new_layer
            module.base_layer.weight.copy_(dequant)
            
            # Update LoRA matrices
            module.lora_A.default.weight.copy_(lora_A)
            module.lora_B.default.weight.copy_(lora_B)

print("saving PISSA adapters")
peft_model.save_pretrained(f"{args.output_dir}/pissa_init")  # Save just the LoRA adapters
print("unloading adapters")
base_model = peft_model.unload()  # This removes the LoRA layers, leaving just the quantized base model
print("saving quantized base model")
base_model.save_pretrained(f"{args.output_dir}/base")  # Save the quantized base model
tokenizer.save_pretrained(f"{args.output_dir}/base")
