// Copyright (c) 2023-2024, Nubificus LTD
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package urunce2etesting

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-ping/ping"
)

func pingUnikernel(ipAddress string) error {
	pinger, err := ping.NewPinger(ipAddress)
	if err != nil {
		return fmt.Errorf("failed to create Pinger: %v", err)
	}
	pinger.Count = 3
	pinger.Timeout = 5 * time.Second
	err = pinger.Run()
	if err != nil {
		return fmt.Errorf("failed to ping %s: %v", ipAddress, err)
	}
	if pinger.PacketsRecv != pinger.PacketsSent {
		return fmt.Errorf("packets received (%d) not equal to packets sent (%d)", pinger.PacketsRecv, pinger.PacketsSent)
	}
	if pinger.PacketsSent == 0 {
		return fmt.Errorf("no packets were sent")
	}
	return nil
}

func verifyNoStaleFiles(containerID string) error {
	// Check /run/containerd/runc/default/containerID directory does not exist
	dirPath := "/run/containerd/runc/default/" + containerID
	_, err := os.Stat(dirPath)
	if !os.IsNotExist(err) {
		return fmt.Errorf("root directory %s still exists", dirPath)
	}

	// Check /run/containerd/runc/k8s.io/containerID directory does not exist
	dirPath = "/run/containerd/runc/k8s.io/" + containerID
	_, err = os.Stat(dirPath)
	if !os.IsNotExist(err) {
		return fmt.Errorf("root directory %s still exists", dirPath)
	}

	// Check /run/containerd/io.containerd.runtime.v2.task/default/containerID directory does not exist
	dirPath = "run/containerd/io.containerd.runtime.v2.task/default/" + containerID
	_, err = os.Stat(dirPath)
	if !os.IsNotExist(err) {
		return fmt.Errorf("bundle directory %s still exists", dirPath)
	}

	// Check /run/containerd/io.containerd.runtime.v2.task/k8s.io/containerID directory does not exist
	dirPath = "run/containerd/io.containerd.runtime.v2.task/k8s.io/" + containerID
	_, err = os.Stat(dirPath)
	if !os.IsNotExist(err) {
		return fmt.Errorf("bundle directory %s still exists", dirPath)
	}

	return nil
}

func findLineInFile(filePath string, pattern string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("Failed to open %s: %v", filePath, err)
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, pattern) {
			return line, nil
		}
	}

	return "", fmt.Errorf("Pattern %s was not found in any line of %s", pattern, filePath)
}