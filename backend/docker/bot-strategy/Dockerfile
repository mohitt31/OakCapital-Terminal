# Dockerfile for the Python strategy execution environment.
# This image will be used as a base for running user-provided scripts.

FROM python:3.11-slim

# Create a non-root user to run the scripts.
RUN useradd --create-home --shell /bin/bash strategy_user

# Set up the working directory.
WORKDIR /home/strategy_user/app

# Install gRPC and other dependencies.
# We are installing the dependencies here in the base image
# to avoid having to do it every time a container is started.
RUN pip install --no-cache-dir grpcio grpcio-tools

# Copy the gRPC server implementation and the generated protobuf files.
# The user script will be mounted into the container at runtime.
COPY --chown=strategy_user:strategy_user . .

# Switch to the non-root user.
USER strategy_user

# Expose the gRPC port.
EXPOSE 50051

# The command to run the gRPC server.
ENTRYPOINT ["python", "grpc_server.py"]
