package kvstore

import (
	"fmt"
	"strings"

	. "github.com/OferMania/clstr-rc/internal/attest"
)

func Equal[T comparable](got, want T, hint string) {
	if got != want {
		panic(fmt.Sprintf("Expected %v, got %v\n\n  %s", want, got, hint))
	}
}

// Define a struct to represent the data
type KeyValueRecord struct {
	Key            string `json:"key"`
	Value          string `json:"value"`
	DurationSecs   uint64 `json:"duration_secs"`
	ExpirationSecs uint64 `json:"expiration_secs"`
	Version        string `json:"version"`
	TableVersion   string `json:"table_version"`
}

type CreateKeyValueRecord struct {
	Value        string `json:"value"`
	DurationSecs uint64 `json:"duration_secs"`
}

func valueToCreateRecord(value string, durationSecs uint64) CreateKeyValueRecord {
	return CreateKeyValueRecord{
		Value:        value,
		DurationSecs: durationSecs,
	}
}

type UpdateKeyValueRecord struct {
	Value        string `json:"value"`
	DurationSecs uint64 `json:"duration_secs"`
	Version      string `json:"version"`
}

func valueToUpdateRecord(value string, version string, durationSecs uint64) UpdateKeyValueRecord {
	return UpdateKeyValueRecord{
		Value:        value,
		DurationSecs: durationSecs,
		Version:      version,
	}
}

type DeleteKeyValueRecord struct {
	Version string `json:"version"`
}

func versionToDeleteRecord(version string) DeleteKeyValueRecord {
	return DeleteKeyValueRecord{
		Version: version,
	}
}

type TableVersionRecord struct {
	TableVersion string `json:"table_version"`
}

func versionToTableVersionRecord(tableVersion string) TableVersionRecord {
	return TableVersionRecord{
		TableVersion: tableVersion,
	}
}

func HTTPAPI() *Suite {
	var updated_tanzania KeyValueRecord

	return New(
		WithCluster(1),
	).
		// 0
		Test("Cleanup", func(do *Do) {
			// In case docker containers are still up from previous runs,
			// we want to clear the store before starting the tests.
			tableInfo := KeyValueRecord{}
			do.GET(Node("n1"), "/table").
				Status(Is(200)).
				Capture(&tableInfo).
				Hint("Your server permit getting on /table for table_version.").
				Run()

			tableReq := versionToTableVersionRecord(tableInfo.TableVersion)
			tableInfo2 := KeyValueRecord{}
			do.DeleteJSON(Node("n1"), "/table", tableReq).
				Status(Is(200)).
				Capture(&tableInfo2).
				Hint("Your server should implement a /table endpoint with table_version,\n" +
					"that deletes all key-value pairs.").
				Run()

		}).

		// 1
		Test("PUT Stores Values", func(do *Do) {
			capitals := map[string]string{
				"kenya":    "Nairobi",
				"uganda":   "Kampala",
				"tanzania": "Dar es Salaam",
			}
			createds := make([]KeyValueRecord, 0)
			for country, capital := range capitals {
				created := KeyValueRecord{}
				req := valueToCreateRecord(capital, 3600)
				// Note: with multi-node checks, captures the last-processed node's response
				do.PutJSON(Node("n1"), fmt.Sprintf("/kv/%s:capital", country), req).
					Status(Is(200)).
					Hint("Your server should accept PUT requests.\n" +
						fmt.Sprintf("req = %s\n", req) +
						"Ensure your HTTP handler processes PUT requests to /kv/{key}.").
					Capture(&created).
					Run()
				createds = append(createds, created)
			}

			dodoma_req1 := valueToCreateRecord("Dodoma", 3600)
			do.PutJSON(Node("n1"), "/kv/tanzania:capital", dodoma_req1).
				Status(Is(400)).
				Hint("Svr must reject invalid updates. Make sure to verify UUID Version").
				Run()

			dodoma_req2 := valueToUpdateRecord("Dodoma", createds[2].Version, 3600)
			updated := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/tanzania:capital", dodoma_req2).
				Status(Is(200)).
				Capture(&updated).
				Hint("Your server should allow overwriting existing keys when correct version UUID is provided.\n" +
					"Ensure PUT requests update the value of existing keys.").
				Run()

			got := KeyValueRecord{}
			do.GET(Node("n1"), "/kv/tanzania:capital").
				Status(Is(200)).
				JSON("value", Is("Dodoma")).
				Capture(&got).
				Hint("Your server should return the updated value after overwrite.\n" +
					"Ensure GET requests return the most recently stored value.").
				Run()

			Equal(got.Version, updated.Version, "Version after update & version from subsequent Get need to match.")

			unicode_req := valueToCreateRecord("🌍 Nairobi", 3600)
			do.PutJSON(Node("n1"), "/kv/unicode:key", unicode_req).
				Status(Is(200)).
				Hint("Your server should handle Unicode characters in values.\n" +
					"Ensure your HTTP handler properly processes UTF-8 encoded data.").
				Run()

			longKey := "long:" + strings.Repeat("k", 100)
			longValue := strings.Repeat("v", 10_000)
			long_req1 := valueToCreateRecord(longValue, 3600)
			do.PutJSON(Node("n1"), fmt.Sprintf("/kv/%s", longKey), long_req1).
				Status(Is(200)).
				Hint("Your server should handle long keys and values.\n" +
					"Ensure your server doesn't have arbitrary key & value length limits.").
				Run()

			spec_req := valueToCreateRecord("value with spaces & symbols! \t", 3600)
			do.PutJSON(Node("n1"), "/kv/special:key-with_symbols.123", spec_req).
				Status(Is(200)).
				Hint("Your server should handle special characters in keys and values.\n" +
					"Ensure proper URL path parsing and value encoding/decoding.").
				Run()

			do.GET(Node("n1"), "/kv/special:key-with_symbols.123").
				Status(Is(200)).
				JSON("value", Is("value with spaces & symbols! \t")).
				Hint("Your server should preserve special characters in stored values.\n" +
					"Ensure proper encoding/decoding doesn't corrupt the data.").
				Run()
		}).

		// 2
		Test("PUT Rejects Empty Keys and Values", func(do *Do) {
			do.PutJSON(Node("n1"), "/kv/empty", valueToCreateRecord("", 3600)).
				Status(Is(400)).
				Body(Matches("value cannot be empty")).
				Hint("Your server should reject empty values.\n" +
					"Add validation to return 400 Bad Request for empty values.").
				Run()

			some_req := valueToCreateRecord("some_value", 3600)
			do.PutJSON(Node("n1"), "/kv/", some_req).
				Status(Is(400)).
				Body(Matches("key cannot be empty")).
				Hint("Your server should reject empty keys.\n" +
					"Add validation to return 400 Bad Request for empty keys.").
				Run()
		}).

		// 3
		Test("GET Returns Stored Values", func(do *Do) {
			do.GET(Node("n1"), "/kv/kenya:capital").
				Status(Is(200)).
				JSON("value", Is("Nairobi")).
				Hint("Your server should return stored values with GET requests.\n" +
					"Ensure your key-value storage and retrieval logic is working correctly.").
				Run()

			do.GET(Node("n1"), "/kv/uganda:capital").
				Status(Is(200)).
				JSON("value", Is("Kampala")).
				Hint("Your server should return stored values with GET requests.\n" +
					"Ensure your key-value storage and retrieval logic is working correctly.").
				Run()

			do.GET(Node("n1"), "/kv/tanzania:capital").
				Status(Is(200)).
				JSON("value", Is("Dodoma")).
				Hint("Your server should return the most recently stored value.\n" +
					"Ensure overwrite operations update the stored value correctly.").
				Capture(&updated_tanzania).
				Run()

			do.GET(Node("n1"), "/kv/unicode:key").
				Status(Is(200)).
				JSON("value", Is("🌍 Nairobi")).
				Hint("Your server should preserve Unicode characters in stored values.\n" +
					"Ensure proper UTF-8 handling in your storage and retrieval logic.").
				Run()

			longKey := "long:" + strings.Repeat("k", 100)
			longValue := strings.Repeat("v", 10_000)
			do.GET(Node("n1"), fmt.Sprintf("/kv/%s", longKey)).
				Status(Is(200)).
				JSON("value", Is(longValue)).
				Hint("Your server should handle retrieval of long keys and values.\n" +
					"Ensure your storage doesn't truncate or corrupt large data.").
				Run()
		}).

		// 4
		Test("GET Rejects Missing and Invalid Keys", func(do *Do) {
			do.GET(Node("n1"), "/kv/nonexistent:key").
				Status(Is(404)).
				Body(Matches("key not found")).
				Hint("Your server should return 404 Not Found when a key doesn't exist.\n" +
					"Check your key lookup logic and error handling.").
				Run()

			do.GET(Node("n1"), "/kv/KENYA:CAPITAL").
				Status(Is(404)).
				Body(Matches("key not found")).
				Hint("Your server should return 404 Not Found when a key doesn't exist.\n" +
					"Check your key lookup logic and error handling.").
				Run()

			do.GET(Node("n1"), "/kv/").
				Status(Is(400)).
				Body(Matches("key cannot be empty")).
				Hint("Your server should reject empty keys.\n" +
					"Add validation to return 400 Bad Request for empty keys.").
				Run()
		}).

		// 5
		Test("DELETE Idempotently Removes Keys", func(do *Do) {
			do.DELETE(Node("n1"), "/kv/tanzania:capital").
				Status(Is(415)).
				Hint("Your server should not permit DELETE requests without version.").
				Run()

			tanzaniaDeleteReq := versionToDeleteRecord(updated_tanzania.Version)
			do.DeleteJSON(Node("n1"), "/kv/tanzania:capital", tanzaniaDeleteReq).
				Status(Is(200)).
				Hint("Your server should accept DELETE requests with a valid version.\n" +
					"Ensure your HTTP handler processes DELETE requests to /kv/{key}.").
				Run()

			do.GET(Node("n1"), "/kv/tanzania:capital").
				Status(Is(404)).
				Body(Matches("key not found")).
				Hint("Your server should return 404 Not Found when a key doesn't exist.\n" +
					"Check your key lookup logic and error handling.").
				Run()

			do.GET(Node("n1"), "/kv/kenya:capital").
				Status(Is(200)).
				JSON("value", Is("Nairobi")).
				Hint("Your server should only delete the specified key, not affect others.\n" +
					"Ensure your delete operation doesn't remove unrelated data.").
				Run()

			do.DELETE(Node("n1"), "/kv/nonexistent:key").
				Status(Is(415)).
				Hint("Delete without JSON is unsupported media type").
				Run()

			twice_record := KeyValueRecord{}
			twice_create_req := valueToCreateRecord("value", 3600)
			do.PutJSON(Node("n1"), "/kv/delete:twice", twice_create_req).
				Status(Is(200)).
				Capture(&twice_record).
				Hint("Your server should accept PUT requests.\n" +
					"Ensure your HTTP handler processes PUT requests to /kv/{key}.").
				Run()

			twice_delete_req := versionToDeleteRecord(twice_record.Version)
			twice_delete_resp := KeyValueRecord{}
			do.DeleteJSON(Node("n1"), "/kv/delete:twice", twice_delete_req).
				Status(Is(200)).
				Capture(&twice_delete_resp).
				Hint("Your server should successfully delete existing keys with version.\n" +
					"Implement proper key removal in your storage logic.").
				Run()

			do.DeleteJSON(Node("n1"), "/kv/delete:twice", versionToDeleteRecord(twice_delete_resp.Version)).
				Status(Is(200)).
				Hint("Your server should handle repeated deletions, 2nd time with version from last time, gracefully.\n" +
					"Deleting the same key twice should be idempotent (return 200 OK).").
				Run()

			twice_create_req = valueToCreateRecord("reinserted", 3600)
			do.PutJSON(Node("n1"), "/kv/delete:twice", twice_create_req).
				Status(Is(200)).
				Hint("Your server should allow re-inserting a previously deleted key.\n" +
					"Ensure your storage doesn't permanently mark keys as deleted.").
				Run()

			do.GET(Node("n1"), "/kv/delete:twice").
				Status(Is(200)).
				JSON("value", Is("reinserted")).
				Hint("Your server should return the new value after re-inserting a deleted key.\n" +
					"Ensure PUT after DELETE works correctly.").
				Run()
		}).

		// 6
		Test("DELETE Rejects Empty Keys", func(do *Do) {
			do.DELETE(Node("n1"), "/kv/").
				Status(Is(400)).
				Body(Matches("key cannot be empty")).
				Hint("Your server should reject empty keys.\n" +
					"Add validation to return 400 Bad Request for empty keys.").
				Run()
		}).

		// 7
		Test("CLEAR Removes All Keys from the Store", func(do *Do) {
			testKeys := map[string]string{
				"clear:test1": "value1",
				"clear:test2": "value2",
				"clear:test3": "value3",
			}
			for key, value := range testKeys {
				do.PutJSON(Node("n1"), fmt.Sprintf("/kv/%s", key), valueToCreateRecord(value, 3600)).
					Status(Is(200)).
					Hint("Your server should accept PUT requests.\n" +
						"Ensure your HTTP handler processes PUT requests to /kv/{key}.").
					Run()
			}

			for key, value := range testKeys {
				do.GET(Node("n1"), fmt.Sprintf("/kv/%s", key)).
					Status(Is(200)).
					JSON("value", Is(value)).
					Hint("Your server should store and retrieve key-value pairs correctly.\n" +
						"Check your storage logic.").
					Run()
			}

			do.DELETE(Node("n1"), "/table").
				Status(Is(415)).
				Hint("Your server prohibits /table without table_version.").
				Run()

			tableInfo := KeyValueRecord{}
			do.GET(Node("n1"), "/table").
				Status(Is(200)).
				Capture(&tableInfo).
				Hint("Your server permit getting on /table for table_version.").
				Run()

			tableReq := versionToTableVersionRecord(tableInfo.TableVersion)
			tableInfo2 := KeyValueRecord{}
			do.DeleteJSON(Node("n1"), "/table", tableReq).
				Status(Is(200)).
				Capture(&tableInfo2).
				Hint("Your server should implement a /table endpoint with table_version,\n" +
					"that deletes all key-value pairs.").
				Run()

			for key := range testKeys {
				do.GET(Node("n1"), fmt.Sprintf("/kv/%s", key)).
					Status(Is(404)).
					Body(Matches("key not found")).
					Hint("Your server should delete all keys when /table is called.\n" +
						"Ensure the /table endpoint removes all stored key-value pairs.").
					Run()
			}

			do.GET(Node("n1"), "/kv/kenya:capital").
				Status(Is(404)).
				Body(Matches("key not found")).
				Hint("Your server should delete ALL keys when /table is called.\n" +
					"Ensure the /table endpoint removes every key-value pair, not just recent ones.").
				Run()

			do.DELETE(Node("n1"), "/table").
				Status(Is(415)).
				Hint("Your server prohibits /table without table_version.").
				Run()

			tableReq2 := versionToTableVersionRecord(tableInfo2.TableVersion)
			do.DeleteJSON(Node("n1"), "/table", tableReq2).
				Status(Is(200)).
				Hint("Your server should handle clearing an empty store gracefully.\n" +
					"Calling /table on an empty store with correct table_version should return 200 OK.").
				Run()
		}).

		// 8
		Test("Concurrent Writes to Different Keys All Succeed", func(do *Do) {
			do.Concurrently(100, func(i int) {
				do.PutJSON(Node("n1"), fmt.Sprintf("/kv/concurrent:key%d", i), valueToCreateRecord(fmt.Sprintf("value%d", i), 3600)).
					Status(Is(200)).
					Hint("Your server should handle concurrent PUT requests.\n" +
						"Ensure thread-safety in your storage implementation.").
					Run()
			})

			for i := 1; i <= 100; i++ {
				do.GET(Node("n1"), fmt.Sprintf("/kv/concurrent:key%d", i)).
					Status(Is(200)).
					JSON("value", Is(fmt.Sprintf("value%d", i))).
					Hint("Your server should store all concurrent writes.\n" +
						"Ensure no data corruption or loss occurs during concurrent operations.").
					Run()
			}
		}).

		// 9
		Test("Concurrent Writes to the Same Key Do Not Corrupt Data", func(do *Do) {
			raceKeyInfo := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/concurrent:racekey", valueToCreateRecord("value", 3600)).
				Status(Is(200)).
				Capture(&raceKeyInfo).
				Hint("Your server should handle concurrent PUT requests.\n" +
					"Ensure thread-safety in your storage implementation.").
				Run()

			// The following test will attempt to concurrently update the same key
			// with different values. Only one of these updates should succeed, and
			// the others should fail with a 409 Conflict.
			statuses := make([]int, 101)
			do.Concurrently(100, func(i int) {
				raceKeyUpdate := valueToUpdateRecord(fmt.Sprintf("value%d", i+1), raceKeyInfo.Version, 3600)
				do.PutJSON(Node("n1"), "/kv/concurrent:racekey", raceKeyUpdate).
					CaptureStatus(&statuses[i]).
					Hint("Your server should handle concurrent PUT requests.\n" +
						"Ensure thread-safety in your storage implementation.").
					Run()
			})

			count200 := 0
			count409 := 0
			index200 := -1
			for i, status := range statuses {
				switch status {
				case 200:
					count200++
					index200 = i
				case 409:
					count409++
				}
			}

			Equal(count200, 1, "Exactly one concurrent write should succeed with 200 OK.")
			Equal(count409, 99, "All other concurrent writes should fail with 409 Conflict.")

			expectedValue := fmt.Sprintf("value%d", index200+1)
			do.GET(Node("n1"), "/kv/concurrent:racekey").
				Status(Is(200)).
				JSON("value", Is(expectedValue)).
				Hint("Your server should handle concurrent writes to the same key.\n" +
					"Ensure thread-safety prevents crashes or data corruption.\n" +
					"The value should be the concurrently written value that passed version validation (value1-value100).").
				Run()
		}).

		// 10
		Test("Unsupported HTTP Methods Return 405", func(do *Do) {
			for _, check := range []*Check{
				do.PATCH(Node("n1"), "/kv/test:key"),
			} {
				check.
					Status(Is(405)).
					Body(Matches("method not allowed")).
					Hint("Your server should reject unsupported HTTP methods on /kv/{key}.\n" +
						"Add logic to return 405 Method Not Allowed for unsupported methods.").
					Run()
			}

			for _, check := range []*Check{
				do.POST(Node("n1"), "/table"),
				do.PUT(Node("n1"), "/table"),
				do.PATCH(Node("n1"), "/table"),
			} {
				check.
					Status(Is(405)).
					Body(Matches("method not allowed")).
					Hint("Your server should reject unsupported HTTP methods on /table.\n" +
						"Only GET or DELETE /table should be allowed. Return 405 Method Not Allowed for other methods.").
					Run()
			}
		})
}
