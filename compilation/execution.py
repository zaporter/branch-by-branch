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

image_name = "branch-by-branch-execution-6"

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
    #execGit(f"git switch -c {branch_name}", repo_dir)
    # create if not exists (I think)
    # ran into issues where when the job failed, it couldn't be retried because the branch already existed
    execGit(f"git checkout -B {branch_name}", repo_dir)

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

original_permissions_map = {}
# Traverses the repo and sets all files to read only, excluding anything in Corelib
def lockdown_permissions(allow_test_lean: bool=False):
    global original_permissions_map
    original_permissions_map = {}

    # if .lake doesnt exist, create it. (important because we are about to make the repo dir read-only)
    if not os.path.exists(os.path.join(repo_dir, ".lake")):
        os.makedirs(os.path.join(repo_dir, ".lake"))

    # Make repo dir read-only
    original_permissions_map[repo_dir] = os.stat(repo_dir).st_mode
    os.chmod(repo_dir, 0o555)


    # Walk through all files in repo directory
    for root, dirs, files in os.walk(repo_dir):
        # Skip Corelib directory
        if "Corelib" in root or ".lake" in root:
            continue
            
        # Make all dirs read-only
        for dir in dirs:
            if "Corelib" in dir or ".lake" in dir:
                continue
            dir_path = os.path.join(root, dir)
            original_permissions_map[dir_path] = os.stat(dir_path).st_mode
            os.chmod(dir_path, 0o555)

        # Make all files read-only
        for file in files:
            if "Corelib" in file or ".lake" in file:
                continue
            if allow_test_lean and file == "Test.lean":
                continue
            file_path = os.path.join(root, file)
            # Remove write permissions for all users
            original_permissions_map[file_path] = os.stat(file_path).st_mode
            os.chmod(file_path, 0o444)

def restore_permissions():
    global original_permissions_map
    for file_path, mode in original_permissions_map.items():
        os.chmod(file_path, mode)

def execute(task: dict) -> dict:
    global container
    if not container:
        raise RuntimeError("Container not initialized")

    print("Executing task")
    print(task)
    results = []


    lockdown_permissions(allow_test_lean=job == "goal-compilation-engine")

    # Execute each pre-command
    hasFailed = False
    for cmd in task["pre_commands"]:
        if hasFailed:
            results.append({
                "action_name": cmd["name"],
                "out": "error: skipped due to previous failure",
                "exit_code": 1
            })
            continue
        print(f"Executing command {cmd['name']}")
        print(f"Command script: {cmd['script']}")
        try:
            exit_code, output = container.exec_run(
                cmd=f"/bin/bash -c '{cmd['script']}'",
                workdir="/home/ubuntu/repo"
            )
        except Exception as e:  
            print(f"Command {cmd['name']} failed with error: {e}")
            exit_code = 1
            output = "error: " + str(e)
        print(f"Command {cmd['name']} exited with code {exit_code}")
        # Hidden commands are allowed to fail
        if exit_code != 0 and not cmd["name"].endswith("hidden"):
            hasFailed = True

        results.append({
            "action_name": cmd["name"],
            "out": type(output) == str and output or output.decode('utf-8'),
            "exit_code": exit_code
        })

    compilation_result = None
    if not hasFailed:
        print(f"Executing compilation script: {task['compilation_script']}")
        try:
            exit_code, output = container.exec_run(
                cmd=f"/bin/bash -c '{task['compilation_script']}'",
                workdir="/home/ubuntu/repo"
            )
        except Exception as e:
            print(f"Compilation script failed with error: {e}")
            exit_code = 1
            output = "error: " + str(e)
        compilation_result = {
            "action_name": "compilation",
            "out": output.decode('utf-8'),
            "exit_code": exit_code
        }
        print(f"Compilation script exited with code {exit_code}")
    else:
        compilation_result = {
            "action_name": "compilation",
            # mimic lean4 output style so it gets stripped correctly
            "out": "error: skipped due to previous failure",
            "exit_code": 1
        }

    restore_permissions()

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
