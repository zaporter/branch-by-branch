import json
import os
import redis
import docker
from typing import Optional

redisHost = os.getenv('REDIS_ADDRESS') or 'err no host'
redisPassword = os.getenv('REDIS_PASSWORD') or 'err no pw'
redisPort = os.getenv('REDIS_PORT') or 'err no port'
r = redis.Redis(host=redisHost, port=int(redisPort), password=redisPassword, decode_responses=True)

dockerClient = docker.from_env()
container: Optional[docker.models.containers.Container] = None

image_name = "branch-by-branch-execution"

def build_image():
    print("Building image")
    dockerClient.images.build(path=".", tag=image_name)

def startup():
    global container
    print("Starting up")
    if not dockerClient.images.list(name=image_name):
        build_image()
        print("Image built")
    # Create a persistent container that we'll reuse
    print("Creating container")
    container = dockerClient.containers.run(
        image=image_name,
        detach=True,  # Run in background
        tty=True,     # Keep container running
        remove=True,  # Remove container when stopped
        volumes={
           # "/home/lean/lean4-execution": {"bind": "/home/lean/lean4-execution", "mode": "rw"},
        },
    )
    print(f"Started container {container.id}")

def shutdown():
    global container
    if container:
        container.stop(timeout=1)
        container = None

def execute(task_msg: dict) -> dict:
    global container
    if not container:
        raise RuntimeError("Container not initialized")

    print("Executing task")
    print(task_msg)
    # Extract the commands to run
    task = json.loads(task_msg)
    results = []

    # Execute each pre-command
    for cmd in task["pre_commands"]:
        print(f"Executing command {cmd['name']}")
        print(f"Command script: {cmd['script']}")
        exit_code, output = container.exec_run(
            cmd=f"/bin/bash -c '{cmd['script']}'",
          #  workdir="/home/lean/lean4-execution"
        )
        print(f"Command {cmd['name']} exited with code {exit_code}")
        results.append({
            "action_name": cmd["name"],
            "out": output.decode('utf-8'),
            "exit_code": exit_code
        })

    print(f"Executing compilation script: {task['compilation_script']}")
    exit_code, output = container.exec_run(
        cmd=f"/bin/bash -c '{task['compilation_script']}'",
      #  workdir="/home/lean/lean4-execution"
    )
    compilation_result = {
        "action_name": "compilation",
        "out": output.decode('utf-8'),
        "exit_code": exit_code
    }
    print(f"Compilation script exited with code {exit_code}")
    return {
        "branch_name": task["branch_name"],
        "pre_commands_results": results,
        "compilation_result": compilation_result
    }

def main():
    try:
        startup()
        while True:
            task = r.brpoplpush("compilation-engine:tasks", "compilation-engine:processing")
            if task:
                task_msg = json.loads(task)
                task_id = task_msg["task_id"]
                inner_task = task_msg["task"]

                result = execute(inner_task)

                result_msg = {
                    "task_id": task_id,
                    "result": json.dumps(result)
                }
                # Store the result back in Redis
                r.lpush("compilation-engine:results", json.dumps(result_msg))
            else:
                print("no tasks, should not be possible to reach here")
                exit(1)
    finally:
        shutdown()

if __name__ == "__main__":
    main()
