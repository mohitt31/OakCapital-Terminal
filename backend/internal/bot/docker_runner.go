package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "synthbull/pkg/gen/strategy"
)

// PythonStrategyRunner is responsible for managing a Docker container for a single Python bot script.
type PythonStrategyRunner struct {
	dockerClient *client.Client
	config       *BotConfig
	containerID  string
	grpcConn     *grpc.ClientConn
	grpcClient   pb.TradingStrategyClient
}

// NewPythonStrategyRunner creates and starts a new Docker container for the given bot config.
func NewPythonStrategyRunner(cfg *BotConfig) (*PythonStrategyRunner, error) {
	if cfg.CustomScriptID == "" {
		return nil, fmt.Errorf("custom script ID is required")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	ctx := context.Background()
	imageName := "python-strategy-base:latest"

	// Pull the image if it doesn't exist.
	// NOTE: Depending on your docker environment, pulling this locally named image
	// from Docker Hub will fail unless you've tagged it from a remote registry.
	// We'll ignore the pull error to allow local images to be used.
	_, _ = cli.ImagePull(ctx, imageName, types.ImagePullOptions{})

	// Create and start the container with resource limits and network isolation.
	// The Docker container is the real security sandbox for user scripts.
	log.Printf("Starting container for script %s...", cfg.CustomScriptID)

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   256 * 1024 * 1024, // 256 MB hard limit
			NanoCPUs: 500_000_000,       // 0.5 CPU
		},
		NetworkMode:    "none", // no network access for user scripts
		ReadonlyRootfs: true,   // prevent writes outside mounted volumes
	}

	// Bind-mount the uploaded script into the container so it runs the
	// user's code rather than the default placeholder.
	if cfg.ScriptPath != "" {
		hostConfig.Binds = []string{
			cfg.ScriptPath + ":/home/strategy_user/app/user_script.py:ro",
		}
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Env:   []string{"SCRIPT_ID=" + cfg.CustomScriptID},
	}, hostConfig, nil, nil, "")

	if err != nil {
		return nil, fmt.Errorf("failed to create docker container: %w", err)
	}

	containerID := resp.ID

	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		// Clean up the created container if start fails
		_ = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get the container's IP address
	inspectData, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		_ = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	ipAddress := inspectData.NetworkSettings.IPAddress
	if ipAddress == "" {
		// Fallback for some network setups (e.g. Docker Desktop on Mac might need localhost mapping)
		ipAddress = "127.0.0.1"
	}

	grpcTarget := fmt.Sprintf("%s:50051", ipAddress)

	// Connect to the gRPC server in the container with retries.
	// The Python gRPC server needs time to boot inside the container.
	var conn *grpc.ClientConn
	for attempt := 0; attempt < 5; attempt++ {
		dialCtx, dialCancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err = grpc.DialContext(dialCtx, grpcTarget,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		dialCancel()
		if err == nil {
			break
		}
		log.Printf("gRPC dial attempt %d/5 for script %s failed: %v", attempt+1, cfg.CustomScriptID, err)
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		// Clean up container if gRPC fails after all retries
		_ = cli.ContainerStop(ctx, containerID, container.StopOptions{})
		_ = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
		return nil, fmt.Errorf("failed to connect to gRPC server after 5 attempts: %w", err)
	}

	return &PythonStrategyRunner{
		dockerClient: cli,
		config:       cfg,
		containerID:  containerID,
		grpcConn:     conn,
		grpcClient:   pb.NewTradingStrategyClient(conn),
	}, nil
}

// OnMarketData sends market data to the Python script and returns its decision.
func (r *PythonStrategyRunner) OnMarketData(tick pb.MarketTick) (*pb.TradeDecision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	decision, err := r.grpcClient.OnMarketData(ctx, &tick)
	if err != nil {
		log.Printf("Error calling OnMarketData for script %s: %v", r.config.CustomScriptID, err)
		return &pb.TradeDecision{Action: pb.TradeDecision_HOLD}, err
	}

	return decision, nil
}

// Stop stops and removes the Docker container.
func (r *PythonStrategyRunner) Stop() {
	log.Printf("Stopping container %s for script %s...", r.containerID, r.config.CustomScriptID)
	if r.grpcConn != nil {
		r.grpcConn.Close()
	}
	if r.dockerClient != nil && r.containerID != "" {
		// Use a background context for cleanup.
		err := r.dockerClient.ContainerStop(context.Background(), r.containerID, container.StopOptions{})
		if err != nil {
			log.Printf("Failed to stop container %s: %v", r.containerID, err)
		} else {
			err = r.dockerClient.ContainerRemove(context.Background(), r.containerID, types.ContainerRemoveOptions{})
			if err != nil {
				log.Printf("Failed to remove container %s: %v", r.containerID, err)
			}
		}
	}
	log.Printf("Container for script %s stopped.", r.config.CustomScriptID)
}
