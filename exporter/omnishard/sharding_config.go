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
	"crypto/md5"
	"errors"
	"math/big"
	"sort"

	omnishardpb "github.com/signalfx/apm-opentelemetry-collector/exporter/omnishard/gen"
)

// shardingInMemConfig is an immutable in-memory representation of sharding
// configuration.
type shardingInMemConfig struct {
	// List of shards sorted by startingHashKey.
	shards []shardInMemConfig
}

// Minimum and maximum possible value for hash key.
var minHashKey = big.NewInt(0)
var maxHashKey = calcMaxHashKey()

// shardInMemConfig  is an immutable in-memory representation of one shard
// configuration.
type shardInMemConfig struct {
	shardID         string
	startingHashKey big.Int
	endingHashKey   big.Int

	// ShardDefinition that was this shard was created from.
	origin *omnishardpb.ShardDefinition
}

// byStartingHashKey implements sort.Interface for []shardInMemConfig based on
// the startingHashKey field.
type byStartingHashKey []shardInMemConfig

func (a byStartingHashKey) Len() int      { return len(a) }
func (a byStartingHashKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byStartingHashKey) Less(i, j int) bool {
	return a[i].startingHashKey.Cmp(&a[j].startingHashKey) < 0
}

// newShardingInMemConfig creates a shardingInMemConfig from omnishardpb.ShardingConfig.
func newShardingInMemConfig(pbConf *omnishardpb.ShardingConfig) (*shardingInMemConfig, error) {
	sc := &shardingInMemConfig{}
	for _, s := range pbConf.ShardDefinitions {
		shard := shardInMemConfig{
			shardID: s.ShardId,
		}
		shard.startingHashKey.SetBytes(s.StartingHashKey)
		shard.endingHashKey.SetBytes(s.EndingHashKey)

		if shard.startingHashKey.Cmp(&shard.endingHashKey) > 0 {
			return nil, errors.New("startingHashKey is bigger than endingHashKey")
		}

		shard.origin = s

		sc.shards = append(sc.shards, shard)
	}

	sort.Sort(byStartingHashKey(sc.shards))

	var prev shardInMemConfig
	for i, shard := range sc.shards {
		if i > 0 && prev.endingHashKey.Cmp(&shard.startingHashKey) >= 0 {
			return nil, errors.New("endingHashKey overlaps with next startingHashKey")
		}
		prev = shard
	}

	return sc, nil
}

func (s *shardInMemConfig) belongsToShard(partitionKey string) bool {
	key := s.partitionKeyToHashKey(partitionKey)
	return key.Cmp(&s.startingHashKey) >= 0 && key.Cmp(&s.endingHashKey) <= 0
}

func (s *shardInMemConfig) partitionKeyToHashKey(partitionKey string) *big.Int {
	b := md5.Sum([]byte(partitionKey))
	return big.NewInt(0).SetBytes(b[:])
}

func calcMaxHashKey() *big.Int {
	var k big.Int
	var buf [md5.Size]byte
	for i := range buf {
		buf[i] = 0xff
	}
	k.SetBytes(buf[:])
	return &k
}
