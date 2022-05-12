/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azurefile

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	utiltesting "k8s.io/client-go/util/testing"
)

func TestSimpleLockEntry(t *testing.T) {
	testLockMap := newLockMap()

	callbackChan1 := make(chan interface{})
	go testLockMap.lockAndCallback(t, "entry1", callbackChan1)
	ensureCallbackHappens(t, callbackChan1)
}

func TestSimpleLockUnlockEntry(t *testing.T) {
	testLockMap := newLockMap()

	callbackChan1 := make(chan interface{})
	go testLockMap.lockAndCallback(t, "entry1", callbackChan1)
	ensureCallbackHappens(t, callbackChan1)
	testLockMap.UnlockEntry("entry1")
}

func TestConcurrentLockEntry(t *testing.T) {
	testLockMap := newLockMap()

	callbackChan1 := make(chan interface{})
	callbackChan2 := make(chan interface{})

	go testLockMap.lockAndCallback(t, "entry1", callbackChan1)
	ensureCallbackHappens(t, callbackChan1)

	go testLockMap.lockAndCallback(t, "entry1", callbackChan2)
	ensureNoCallback(t, callbackChan2)

	testLockMap.UnlockEntry("entry1")
	ensureCallbackHappens(t, callbackChan2)
	testLockMap.UnlockEntry("entry1")
}

func (lm *lockMap) lockAndCallback(t *testing.T, entry string, callbackChan chan<- interface{}) {
	lm.LockEntry(entry)
	callbackChan <- true
}

var callbackTimeout = 2 * time.Second

func ensureCallbackHappens(t *testing.T, callbackChan <-chan interface{}) bool {
	select {
	case <-callbackChan:
		return true
	case <-time.After(callbackTimeout):
		t.Fatalf("timed out waiting for callback")
		return false
	}
}

func ensureNoCallback(t *testing.T, callbackChan <-chan interface{}) bool {
	select {
	case <-callbackChan:
		t.Fatalf("unexpected callback")
		return false
	case <-time.After(callbackTimeout):
		return true
	}
}

func TestUnlockEntryNotExists(t *testing.T) {
	testLockMap := newLockMap()

	callbackChan1 := make(chan interface{})
	go testLockMap.lockAndCallback(t, "entry1", callbackChan1)
	ensureCallbackHappens(t, callbackChan1)
	// entry2 does not exist
	testLockMap.UnlockEntry("entry2")
	testLockMap.UnlockEntry("entry1")
}

func TestIsDiskFsType(t *testing.T) {
	tests := []struct {
		fsType         string
		expectedResult bool
	}{
		{
			fsType:         "ext4",
			expectedResult: true,
		},
		{
			fsType:         "ext3",
			expectedResult: true,
		},
		{
			fsType:         "ext2",
			expectedResult: true,
		},
		{
			fsType:         "xfs",
			expectedResult: true,
		},
		{
			fsType:         "",
			expectedResult: false,
		},
		{
			fsType:         "cifs",
			expectedResult: false,
		},
		{
			fsType:         "invalid",
			expectedResult: false,
		},
	}

	for _, test := range tests {
		result := isDiskFsType(test.fsType)
		if result != test.expectedResult {
			t.Errorf("isDiskFsType(%s) returned with %v, not equal to %v", test.fsType, result, test.expectedResult)
		}
	}
}

func TestIsSupportedShareNamePrefix(t *testing.T) {
	tests := []struct {
		prefix         string
		expectedResult bool
	}{
		{
			prefix:         "",
			expectedResult: true,
		},
		{
			prefix:         "ext3",
			expectedResult: true,
		},
		{
			prefix:         "ext-2",
			expectedResult: true,
		},
		{
			prefix:         "-xfs",
			expectedResult: false,
		},
		{
			prefix:         "Absdf",
			expectedResult: false,
		},
		{
			prefix:         "tooooooooooooooooooooooooolong",
			expectedResult: false,
		},
		{
			prefix:         "+invalid",
			expectedResult: false,
		},
		{
			prefix:         " invalidspace",
			expectedResult: false,
		},
	}

	for _, test := range tests {
		result := isSupportedShareNamePrefix(test.prefix)
		if result != test.expectedResult {
			t.Errorf("isSupportedShareNamePrefix(%s) returned with %v, not equal to %v", test.prefix, result, test.expectedResult)
		}
	}
}

func TestIsSupportedFsType(t *testing.T) {
	tests := []struct {
		fsType         string
		expectedResult bool
	}{
		{
			fsType:         "ext4",
			expectedResult: true,
		},
		{
			fsType:         "ext3",
			expectedResult: true,
		},
		{
			fsType:         "ext2",
			expectedResult: true,
		},
		{
			fsType:         "xfs",
			expectedResult: true,
		},
		{
			fsType:         "",
			expectedResult: true,
		},
		{
			fsType:         "cifs",
			expectedResult: true,
		},
		{
			fsType:         "smb",
			expectedResult: true,
		},
		{
			fsType:         "invalid",
			expectedResult: false,
		},
	}

	for _, test := range tests {
		result := isSupportedFsType(test.fsType)
		if result != test.expectedResult {
			t.Errorf("isSupportedFsType(%s) returned with %v, not equal to %v", test.fsType, result, test.expectedResult)
		}
	}
}

func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		desc         string
		rpcErr       error
		expectedBool bool
	}{
		{
			desc:         "non-retriable error",
			rpcErr:       nil,
			expectedBool: false,
		},
		{
			desc:         "accountNotProvisioned",
			rpcErr:       errors.New("could not get storage key for storage account : could not get storage key for storage account f233333: Retriable: true, RetryAfter: 0001-01-01 00:00:00 +0000 UTC, HTTPStatusCode: 409, RawError: storage.AccountsClient#ListKeys: Failure sending request: StatusCode=409 -- Original Error: autorest/azure: Service returned an error. Status=<nil> Code=\"StorageAccountIsNotProvisioned\" Message=\"The storage account provisioning state must be 'Succeeded' before executing the operation.\""),
			expectedBool: true,
		},
		{
			desc:         "tooManyRequests",
			rpcErr:       errors.New("could not get storage key for storage account : could not list storage accounts for account type Premium_LRS: Retriable: true, RetryAfter: 0001-01-01 00:00:00 +0000 UTC m=+231.866923225, HTTPStatusCode: 429, RawError: storage.AccountsClient#ListByResourceGroup: Failure responding to request: StatusCode=429 -- Original Error: autorest/azure: Service returned an error. Status=429 Code=\"TooManyRequests\" Message=\"The request is being throttled as the limit has been reached for operation type - List. For more information, see - https://aka.ms/srpthrottlinglimits\""),
			expectedBool: true,
		},
		{
			desc:         "shareBeingDeleted",
			rpcErr:       errors.New("storage.FileSharesClient#Create: Failure sending request: StatusCode=409 -- Original Error: autorest/azure: Service returned an error. Status=<nil> Code=\"ShareBeingDeleted\" Message=\"The specified share is being deleted. Try operation later.\""),
			expectedBool: true,
		},
		{
			desc:         "clientThrottled",
			rpcErr:       errors.New("could not list storage accounts for account type : Retriable: true, RetryAfter: 16s, HTTPStatusCode: 0, RawError: azure cloud provider throttled for operation StorageAccountListByResourceGroup with reason \"client throttled\""),
			expectedBool: true,
		},
	}

	for _, test := range tests {
		result := isRetriableError(test.rpcErr)
		if result != test.expectedBool {
			t.Errorf("desc: (%s), input: rpcErr(%v), isRetriableError returned with bool(%v), not equal to expectedBool(%v)",
				test.desc, test.rpcErr, result, test.expectedBool)
		}
	}
}

func TestSleepIfThrottled(t *testing.T) {
	start := time.Now()
	sleepIfThrottled(errors.New("tooManyRequests"), 10)
	elapsed := time.Since(start)
	if elapsed.Seconds() < 10 {
		t.Errorf("Expected sleep time(%d), Actual sleep time(%f)", 10, elapsed.Seconds())
	}

}

func TestUseDataPlaneAPI(t *testing.T) {
	volumeContext := map[string]string{"usedataplaneapi": "true"}
	result := useDataPlaneAPI(volumeContext)
	if result != true {
		t.Errorf("Expected value(%s), Actual value(%t)", "true", result)
	}
}

func TestCreateStorageAccountSecret(t *testing.T) {
	result := createStorageAccountSecret("TestAccountName", "TestAccountKey")
	if result[defaultSecretAccountName] != "TestAccountName" || result[defaultSecretAccountKey] != "TestAccountKey" {
		t.Errorf("Expected account name(%s), Actual account name(%s); Expected account key(%s), Actual account key(%s)", "TestAccountName", result[defaultSecretAccountName], "TestAccountKey", result[defaultSecretAccountKey])
	}
}

func TestConvertTagsToMap(t *testing.T) {
	tests := []struct {
		desc          string
		tags          string
		expectedError error
	}{
		{desc: "Invalid tag",
			tags:          "invalid=test=tag",
			expectedError: errors.New("Tags 'invalid=test=tag' are invalid, the format should like: 'key1=value1,key2=value2'"),
		},
		{desc: "Invalid key",
			tags:          "=test",
			expectedError: errors.New("Tags '=test' are invalid, the format should like: 'key1=value1,key2=value2'"),
		},
		{desc: "Valid tags",
			tags:          "testTag=testValue",
			expectedError: nil},
	}

	for _, test := range tests {
		_, err := ConvertTagsToMap(test.tags)
		if !reflect.DeepEqual(err, test.expectedError) {
			t.Errorf("test[%s]: unexpected error: %v, expected error: %v", test.desc, err, test.expectedError)
		}
	}
}

func TestSetVolumeOwnership(t *testing.T) {
	tmpVDir, err := utiltesting.MkTmpdir("SetVolumeOwnership")
	if err != nil {
		t.Fatalf("can't make a temp dir: %v", err)
	}
	//deferred clean up
	defer os.RemoveAll(tmpVDir)

	tests := []struct {
		path          string
		gid           string
		expectedError error
	}{
		{
			path:          "path",
			gid:           "",
			expectedError: fmt.Errorf("convert %s to int failed with %v", "", `strconv.Atoi: parsing "": invalid syntax`),
		},
		{
			path:          "path",
			gid:           "alpha",
			expectedError: fmt.Errorf("convert %s to int failed with %v", "alpha", `strconv.Atoi: parsing "alpha": invalid syntax`),
		},
		{
			path:          "not-exists",
			gid:           "1000",
			expectedError: fmt.Errorf("lstat not-exists: no such file or directory"),
		},
		{
			path:          tmpVDir,
			gid:           "1000",
			expectedError: nil,
		},
	}

	for _, test := range tests {
		err := SetVolumeOwnership(test.path, test.gid)
		if err != nil && (err.Error() != test.expectedError.Error()) {
			t.Errorf("unexpected error: %v, expected error: %v", err, test.expectedError)
		}
	}
}
