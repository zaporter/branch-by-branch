# branch-by-branch

## Startup flow
[RDB, Gitea] -> [Orchestrator] -> [Compilation engines, Inference engines, Training]
(where [] indicates a parallelizable group)
It is especially important to start the orchestrator before the training server.

## Model prep
Run ./training/run_training.sh init_qpissa.py {args}
Then push the model & adapter to b2. 

My infra is based around qpissa. You may have to make modifications to get another adapter scheme to work well.

## Gitea

Git clone command:
```sh
git clone ssh://root@hetzner:port/zaporter/byb-v1.git
```
(This is private for now.)

## lean_corelib

Starting spot for the llm to expand.

"core" because this is the core data structures for math. Not because anyone should use or depend on this.
Renamed from Mathlib as a thin veil to help push the llm away from duplicating Mathlib.

This is a subset of Mathlib master@#9837ca9 (v4.15.0)


