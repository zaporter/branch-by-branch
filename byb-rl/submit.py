from ray.job_submission import JobSubmissionClient
client = JobSubmissionClient("https://anvil.werewolf-banded.ts.net/cluster/zack/test/")

job_id = client.submit_job(
    entrypoint="uv run deepspeed_torch_basic.py",
    runtime_env={
        "working_dir": "./",
        "env_vars": {
            "RAY_ENABLE_RECORD_ACTOR_TASK_LOGGING": "1",
        },
    },
)
print(f"Submitted job with ID: {job_id}")