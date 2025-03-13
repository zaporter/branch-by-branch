FROM ubuntu:24.04
# This dockerfile is responsible for isolating script execution from the rest of the system (that is running execution.py)

ENV LEAN_TOOLCHAIN_VERSION="v4.15.0"

RUN apt-get update && apt-get install -y curl git elan ed

# Ignoring instructions on https://leanprover-community.github.io/install/linux.html
# Installing elan via apt is easier.
# If this fails, see https://github.com/Seasawher/docker-lean4/blob/main/Dockerfile
RUN elan toolchain install ${LEAN_TOOLCHAIN_VERSION}
RUN elan default ${LEAN_TOOLCHAIN_VERSION}

# Create working directory
WORKDIR /home/ubuntu

# Keep container running
CMD ["tail", "-f", "/dev/null"]

