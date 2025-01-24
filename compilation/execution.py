import json
import os
import random
import redis
import docker
from typing import Optional
import subprocess

redisHost = os.getenv('REDIS_ADDRESS') or 'err no host'
redisPassword = os.getenv('REDIS_PASSWORD') or 'err no pw'
redisPort = os.getenv('REDIS_PORT') or 'err no port'
r = redis.Redis(host=redisHost, port=int(redisPort), password=redisPassword, decode_responses=True)

dockerClient = docker.from_env()
container: Optional[docker.models.containers.Container] = None

image_name = "branch-by-branch-execution"

repo_dir = os.getenv("REPO_DIR") or "repo"

params=None

def update_params():
    global params
    params = {
        "repo_url": r.get("execution:repo_url"),
        "compilation_command": r.get("execution:compilation_command"),
    }

def execGit(cmd: str, cwd: str | None):
    env = os.environ.copy()
    env["GIT_SSH_COMMAND"] = f"ssh -i {os.getenv('ROUTER_SSH_KEY')}"
    # the shell scripts never end. 🧅
    result = subprocess.run(["/bin/sh", "-c", cmd], env=env, cwd=cwd, capture_output=True, text=True)
    print(f"execGit {cmd} result: {result.returncode}")
    print(result.stdout)
    print(result.stderr)
    if result.returncode != 0:
        raise RuntimeError(f"Failed to execute git command {cmd} in repo {repo_dir}: {result.stderr}")

def git_clone_repo(repo_url: str):
    if os.path.exists(repo_dir):
        git_pull()
    else:
        execGit(f"git clone {repo_url} {repo_dir}", os.getcwd())

def git_pull():
    execGit("git pull", repo_dir)

def git_create_branch(branch_name: str):
    execGit(f"git switch -c {branch_name}", repo_dir)

def git_checkout(branch_name: str):
    execGit(f"git pull origin {branch_name} && git checkout {branch_name}", repo_dir)

def git_push(branch_name: str):
    execGit(f"git push origin {branch_name}", repo_dir)

def git_commit(branch_name: str, commit_msg: str):
    execGit(f"git add . && git commit -m {commit_msg}", repo_dir)

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
    print("cloning repo")
    git_clone_repo(params["repo_url"])
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

def testGit():
    git_clone_repo(params["repo_url"])
    branch_name = "test" + str(random.randint(0, 1000000))
    git_create_branch(branch_name)
    ret = os.system(f"echo 'test' > {repo_dir}/test.txt")
    if ret != 0:
        raise RuntimeError("Failed to create test.txt")
    git_commit(branch_name, "branch")
    git_push(branch_name)
    git_checkout("main")

if __name__ == "__main__":
    update_params()
    testGit()
