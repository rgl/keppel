/*******************************************************************************
*
* Copyright 2021 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package clair

import "testing"

func TestMergeVulnerabilityStatuses(t *testing.T) {
	expect := func(expected, actual VulnerabilityStatus) {
		t.Helper()
		if expected != actual {
			t.Errorf("expected %s, but got %s", expected, actual)
		}
	}
	expect(CleanSeverity, MergeVulnerabilityStatuses())
	expect(CleanSeverity, MergeVulnerabilityStatuses(CleanSeverity))

	expect(ErrorVulnerabilityStatus, MergeVulnerabilityStatuses(ErrorVulnerabilityStatus))
	expect(ErrorVulnerabilityStatus, MergeVulnerabilityStatuses(ErrorVulnerabilityStatus, PendingVulnerabilityStatus))
	expect(ErrorVulnerabilityStatus, MergeVulnerabilityStatuses(ErrorVulnerabilityStatus, UnsupportedVulnerabilityStatus))
	expect(ErrorVulnerabilityStatus, MergeVulnerabilityStatuses(PendingVulnerabilityStatus, ErrorVulnerabilityStatus))
	expect(ErrorVulnerabilityStatus, MergeVulnerabilityStatuses(ErrorVulnerabilityStatus, HighSeverity))

	expect(UnsupportedVulnerabilityStatus, MergeVulnerabilityStatuses(UnsupportedVulnerabilityStatus))
	expect(UnsupportedVulnerabilityStatus, MergeVulnerabilityStatuses(UnsupportedVulnerabilityStatus, HighSeverity))
	expect(UnsupportedVulnerabilityStatus, MergeVulnerabilityStatuses(HighSeverity, UnsupportedVulnerabilityStatus))
	expect(UnsupportedVulnerabilityStatus, MergeVulnerabilityStatuses(PendingVulnerabilityStatus, UnsupportedVulnerabilityStatus))

	expect(PendingVulnerabilityStatus, MergeVulnerabilityStatuses(PendingVulnerabilityStatus))
	expect(PendingVulnerabilityStatus, MergeVulnerabilityStatuses(PendingVulnerabilityStatus, HighSeverity))
	expect(PendingVulnerabilityStatus, MergeVulnerabilityStatuses(HighSeverity, PendingVulnerabilityStatus))

	expect(LowSeverity, MergeVulnerabilityStatuses(LowSeverity, LowSeverity))
	expect(HighSeverity, MergeVulnerabilityStatuses(LowSeverity, HighSeverity))
}
