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

package unikernels

import (
	"fmt"
	"strings"
)

const MewzUnikernel string = "mewz"

type Mewz struct {
	Command string
	Net     MewzNet
}

type MewzNet struct {
	Address string
	Mask    string
	Gateway string
}

func (m *Mewz) CommandString() (string, error) {
	return fmt.Sprintf("%s/%s %s ", m.Net.Address, m.Net.Mask,
		m.Net.Gateway), nil
}

func (m *Mewz) SupportsBlock() bool {
	return false
}

func (m *Mewz) SupportsFS(_ string) bool {
	return false
}

func (m *Mewz) Init(data UnikernelParams) error {
	m.Command = strings.TrimSpace(data.CmdLine)
	m.Net.Address = data.EthDeviceIP
	m.Net.Mask = "24"
	m.Net.Gateway = data.EthDeviceGateway

	return nil
}

func newMewz() *Mewz {
	mewzStruct := new(Mewz)
	return mewzStruct
}
