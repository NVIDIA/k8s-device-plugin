/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitKeepsIDWhenReplicaIsNotAnInteger(t *testing.T) {
	id, replica := AnnotatedID("GPU-abc::nope").Split()
	require.Equal(t, "GPU-abc::nope", id)
	require.Equal(t, 0, replica)
	require.False(t, AnnotatedID("GPU-abc::nope").HasAnnotations())
	require.Equal(t, "GPU-abc::nope", AnnotatedID("GPU-abc::nope").GetID())

	id, replica = AnnotatedID("GPU-abc::1").Split()
	require.Equal(t, "GPU-abc", id)
	require.Equal(t, 1, replica)
	require.True(t, AnnotatedID("GPU-abc::1").HasAnnotations())

	id, replica = AnnotatedID("GPU-abc").Split()
	require.Equal(t, "GPU-abc", id)
	require.Equal(t, 0, replica)
	require.False(t, AnnotatedID("GPU-abc").HasAnnotations())
}
