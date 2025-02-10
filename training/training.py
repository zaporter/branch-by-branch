from peft import LoraConfig, get_peft_model
from transformers import AutoModelForCausalLM, AutoTokenizer
import torch
import transformers


model = AutoModelForCausalLM.from_pretrained(
    model_name="gpt2",
    # Do I need to do something to support unsloth / quantization?
    trust_remote_code=True
)

lora_config = LoraConfig(
    # https://huggingface.co/docs/peft/en/developer_guides/lora
    # init_lora_weights="pissa"
    r=16,
    lora_alpha=32,
    target_modules = ["q_proj", "k_proj", "v_proj", "out_proj", "fc_in", "fc_out", "wte"],
    lora_dropout=0.1,
    bias="none",
    task_type="CAUSAL_LM"
)
# https://huggingface.co/docs/peft/en/quicktour
model = get_peft_model(model, lora_config)
model.print_trainable_parameters()

def main():
    print("Hello World")

# TODO:
# https://github.com/huggingface/trl/blob/55e680e142d88e090dcbf5a469eab1ebba28ddef/trl/trainer/grpo_trainer.py#L625
def compute_loss():
    pass

if __name__ == "__main__":
    main()
