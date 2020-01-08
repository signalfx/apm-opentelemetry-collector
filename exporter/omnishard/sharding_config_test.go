// Copyright 2019 Omnition Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package omnishard

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	omnishardpb "github.com/signalfx/apm-opentelemetry-collector/exporter/omnishard/gen"
)

func TestNewShardingInMemConfig(t *testing.T) {
	pbConf := &omnishardpb.ShardingConfig{
		ShardDefinitions: []*omnishardpb.ShardDefinition{
			{
				ShardId:         "def",
				StartingHashKey: big.NewInt(10).Bytes(),
				EndingHashKey:   big.NewInt(20).Bytes(),
			},
			{
				ShardId:         "abc",
				StartingHashKey: big.NewInt(0).Bytes(),
				EndingHashKey:   big.NewInt(9).Bytes(),
			},
		},
	}

	sc, err := newShardingInMemConfig(pbConf)
	assert.Nil(t, err)

	want := &shardingInMemConfig{
		shards: []shardInMemConfig{
			{
				shardID: "abc",
				origin:  pbConf.ShardDefinitions[1],
			},
			{
				shardID: "def",
				origin:  pbConf.ShardDefinitions[0],
			},
		},
	}
	want.shards[0].startingHashKey.SetUint64(0)
	want.shards[0].endingHashKey.SetUint64(9)

	want.shards[1].startingHashKey.SetUint64(10)
	want.shards[1].endingHashKey.SetUint64(20)

	assert.Equal(t, sc, want)
}
