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

image_name = "branch-by-branch-execution-1"

repo_dir = os.getenv("REPO_DIR") or "repo"
job = os.getenv("JOB")
jobs = ["compilation-engine", "goal-compilation-engine"]
if job not in jobs:
    raise RuntimeError(f"Invalid job: {job}. Must be one of: {jobs}")

params=None

def update_params():
    global params
    params = {
        "repo_url": r.get("execution:repo_url"),
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
        execGit("git switch main", repo_dir)
        git_pull()
    else:
        execGit(f"git clone {repo_url} {repo_dir}", os.getcwd())

def git_pull():
    execGit("git pull", repo_dir)

def git_clean():
    execGit("git clean -fd", repo_dir)

def git_create_branch(branch_name: str):
    execGit(f"git switch -c {branch_name}", repo_dir)

def git_checkout(branch_name: str):
    execGit(f"git fetch origin {branch_name}", repo_dir)
    execGit(f"git checkout {branch_name}", repo_dir)
    execGit(f"git pull origin {branch_name} --ff-only", repo_dir)

def git_push(branch_name: str):
    execGit(f"git push origin {branch_name}", repo_dir)

def git_commit(commit_msg: str):
    execGit(f"git add . && git commit -m {commit_msg} --allow-empty", repo_dir)

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

    volumes={
        repo_dir: {"bind": "/home/ubuntu/repo", "mode": "rw"},
        # Problem engine needs to write new tests
        repo_dir+"/Test.lean": {
            "bind": "/home/ubuntu/repo/Test.lean",
            "mode": "rw" if job == "goal-compilation-engine" else "ro"
        },
        # Hardcoded read only sub-dirs 
        repo_dir+"/lake-manifest.json": {"bind": "/home/ubuntu/repo/lake-manifest.json", "mode": "ro"},
        repo_dir+"/lakefile.toml": {"bind": "/home/ubuntu/repo/lakefile.toml", "mode": "ro"},
        repo_dir+"/lean-toolchain": {"bind": "/home/ubuntu/repo/lean-toolchain", "mode": "ro"},
        repo_dir+"/.gitignore": {"bind": "/home/ubuntu/repo/.gitignore", "mode": "ro"},
        repo_dir+"/mk_all.lean": {"bind": "/home/ubuntu/repo/mk_all.lean", "mode": "ro"},
    }
    print(f"Volumes: {volumes}")

    container = dockerClient.containers.run(
        image=image_name,
        detach=True,  # Run in background
        tty=True,     # Keep container running
        remove=True,  # Remove container when stopped
        user="ubuntu",
        volumes=volumes,
    )
    print(f"Started container {container.id}")

def shutdown():
    global container
    if container:
        container.stop(timeout=1)
        container = None

def execute(task: dict) -> dict:
    global container
    if not container:
        raise RuntimeError("Container not initialized")

    print("Executing task")
    print(task)
    results = []

    # Execute each pre-command
    hasFailed = False
    for cmd in task["pre_commands"]:
        if hasFailed:
            results.append({
                "action_name": cmd["name"],
                "out": "skipped due to previous failure",
                "exit_code": 1
            })
            continue
        print(f"Executing command {cmd['name']}")
        print(f"Command script: {cmd['script']}")
        exit_code, output = container.exec_run(
            cmd=f"/bin/bash -c '{cmd['script']}'",
            workdir="/home/ubuntu/repo"
        )
        print(f"Command {cmd['name']} exited with code {exit_code}")
        # Hidden commands are allowed to fail
        if exit_code != 0 and not cmd["name"].endswith("hidden"):
            hasFailed = True

        results.append({
            "action_name": cmd["name"],
            "out": output.decode('utf-8'),
            "exit_code": exit_code
        })

    compilation_result = None
    if not hasFailed:
        print(f"Executing compilation script: {task['compilation_script']}")
        exit_code, output = container.exec_run(
            cmd=f"/bin/bash -c '{task['compilation_script']}'",
            workdir="/home/ubuntu/repo"
        )
        compilation_result = {
            "action_name": "compilation",
            "out": output.decode('utf-8'),
            "exit_code": exit_code
        }
        print(f"Compilation script exited with code {exit_code}")
    else:
        compilation_result = {
            "action_name": "compilation",
            "out": "skipped due to previous failure",
            "exit_code": 1
        }

    return {
        "pre_commands_results": results,
        "compilation_result": compilation_result
    }

def main():
    try:
        startup()
        while True:
            task = r.brpoplpush(f"{job}:tasks", f"{job}:processing")
            if task:
                try:
                    task_msg = json.loads(task)
                    task_id = task_msg["task_id"]
                    compilation_task = json.loads(task_msg["task"])
                    old_branch_name = compilation_task["branch_name"]
                    new_branch_name = compilation_task["new_branch_name"]
                    git_checkout(old_branch_name)
                    git_create_branch(new_branch_name)
                    git_clean()
                    result = execute(compilation_task)
                    git_commit("compilation")
                    git_push(new_branch_name)

                    result["branch_name"] = new_branch_name

                    result_msg = {
                        "task_id": task_id,
                        "result": json.dumps(result)
                    }
                    # Store the result back in Redis
                    r.lpush(f"{job}:results", json.dumps(result_msg))
                except Exception as e:
                    # Fine -- it will be requeued.
                    print(f"Error executing task {task_id}: {e}")
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
    git_commit("compilation")
    git_push(branch_name)
    git_checkout("main")

if __name__ == "__main__":
    update_params()
    main()
