package kvstore

import (
	"fmt"
	"strings"
	"time"

	. "github.com/OferMania/clstr-rc/internal/attest"
)

func Equal[T comparable](got, want T, hint string) {
	if got != want {
		panic(fmt.Sprintf("Expected %v, got %v\n\n  %s", want, got, hint))
	}
}

func NotEqual[T comparable](got, want T, hint string) {
	if got == want {
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
			createds := make(map[string]KeyValueRecord)
			for country, capital := range capitals {
				created := KeyValueRecord{}
				req := valueToCreateRecord(capital, 3600)
				do.PutJSON(Node("n1"), fmt.Sprintf("/kv/%s:capital", country), req).
					Status(Is(200)).
					Hint("Your server should accept PUT requests.\n" +
						fmt.Sprintf("req = %s\n", req) +
						"Ensure your HTTP handler processes PUT requests to /kv/{key}.").
					Capture(&created).
					Run()
				createds[country] = created
			}

			dodoma_req1 := valueToCreateRecord("Dodoma", 3600)
			do.PutJSON(Node("n1"), "/kv/tanzania:capital", dodoma_req1).
				Status(Is(400)).
				Hint("Svr must reject invalid updates. Make sure to verify UUID Version").
				Run()

			dodoma_req2 := valueToUpdateRecord("Dodoma", createds["tanzania"].Version, 3600)
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
			do.PutJSON(Node("n1"), "/kv/empty", map[string]any{"duration_secs": 3600}).
				Status(Is(400)).
				Body(Matches("new key 'empty' requires a value on initialization")).
				Hint("Your server should reject omitted value field.\n" +
					"Add validation to return 400 Bad Request for empty values.").
				Run()

			do.PutJSON(Node("n1"), "/kv/empty", valueToCreateRecord("", 3600)).
				Status(Is(400)).
				Body(Matches("new key 'empty' requires a value on initialization")).
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

			// set value to nonempty, then update without a value
			got1 := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/empty", valueToCreateRecord("nonempty", 3600)).
				Status(Is(200)).
				Capture(&got1).
				Hint("Your server should accept nonempty values for the key.").
				Run()
			update_req1 := valueToUpdateRecord("", got1.Version, 3600)
			got2 := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/empty", update_req1).
				Status(Is(200)).
				Capture(&got2).
				Hint("Your server should permit updating with empty values.").
				Run()
			Equal(got2.Value, "nonempty", "Value needs to match nonempty.")
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
		Test("DELETE Removes Keys Once", func(do *Do) {
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

			do.DeleteJSON(Node("n1"), "/kv/nonexistent:key", versionToDeleteRecord(updated_tanzania.Version)).
				Status(Is(409)).
				Hint("Even with JSON with non-zero UUID, deletion on non-existent key is prohibited").
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
				Status(Is(400)).
				Hint("Repeated deletions should be banned since key no longer exists.\n" +
					"Deleting the same key twice on valid UUID should give 400.").
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
		}).

		// 11
		Test("Expired Keys Are Removed", func(do *Do) {
			req1 := valueToCreateRecord("temp-value1", 2)    // 2-second expiration
			req2 := valueToCreateRecord("temp-value2", 7200) // 2-hour expiration
			req3 := valueToCreateRecord("temp-value3", 120)  // 2-minute expiration
			req4 := valueToCreateRecord("temp-value4", 0)    // never expires
			do.PutJSON(Node("n1"), "/kv/expiring:key1", req1).
				Status(Is(200)).
				Hint("Server should accept a short-lived key.").
				Run()
			do.PutJSON(Node("n1"), "/kv/expiring:key2", req2).
				Status(Is(200)).
				Hint("Server should accept a long-lived key.").
				Run()
			do.PutJSON(Node("n1"), "/kv/expiring:key3", req3).
				Status(Is(200)).
				Hint("Server should accept a medium-lived key.").
				Run()
			do.PutJSON(Node("n1"), "/kv/expiring:key4", req4).
				Status(Is(200)).
				Hint("Server should accept a never-expiring key.").
				Run()

			record1 := KeyValueRecord{}
			do.GET(Node("n1"), "/kv/expiring:key1").
				Status(Is(200)).
				Capture(&record1).
				Hint("Key should exist immediately after creation.").
				Run()

			Equal(record1.Value, "temp-value1", "Value for expiring:key1 should match the stored value.")
			Equal(record1.DurationSecs, uint64(2), "DurationSecs for expiring:key1 should match the stored duration.")

			do.GET(Node("n1"), "/kv/expiring:key1").
				Eventually(5 * time.Second).
				Status(Is(404)).
				Hint("Server's background cleaner should remove the key within a few seconds of expiration.").
				Run()
			record2 := KeyValueRecord{}
			do.GET(Node("n1"), "/kv/expiring:key2").
				Status(Is(200)).
				Capture(&record2).
				Hint("Long-lived Key should exist well after creation.").
				Run()
			record3a := KeyValueRecord{}
			do.GET(Node("n1"), "/kv/expiring:key3").
				Status(Is(200)).
				Capture(&record3a).
				Hint("Medium-lived Key should exist well after creation.").
				Run()
			record4 := KeyValueRecord{}
			do.GET(Node("n1"), "/kv/expiring:key4").
				Status(Is(200)).
				Capture(&record4).
				Hint("Never-expiring Key should exist well after creation.").
				Run()

			Equal(record2.Value, "temp-value2", "Value for expiring:key2 should match the stored value.")
			Equal(record2.DurationSecs, uint64(7200), "DurationSecs for expiring:key2 should match the stored duration.")

			Equal(record3a.Value, "temp-value3", "Value for expiring:key3 should match the stored value.")
			Equal(record3a.DurationSecs, uint64(120), "DurationSecs for expiring:key3 should match the stored duration.")

			Equal(record4.Value, "temp-value4", "Value for expiring:key4 should match the stored value.")
			Equal(record4.DurationSecs, uint64(0), "DurationSecs for expiring:key4 should match the stored duration.")

			req3b := valueToUpdateRecord("temp-value3-updated", record3a.Version, 130)
			do.PutJSON(Node("n1"), "/kv/expiring:key3", req3b).
				Status(Is(200)).
				Hint("Server should allow updating an expiring key.").
				Run()
			record3b := KeyValueRecord{}
			do.GET(Node("n1"), "/kv/expiring:key3").
				Status(Is(200)).
				Capture(&record3b).
				Hint("Updated Key should exist well after update.").
				Run()

			Equal(record3b.Value, "temp-value3-updated", "Value for expiring:key3 should match the updated value.")
			Equal(record3b.DurationSecs, uint64(130), "DurationSecs for expiring:key3 should match the updated duration.")
			NotEqual(record3a.Version, record3b.Version, "Version for expiring:key3 should change after update.")
		}).

		// 12
		Test("Omitted Duration Gets Default TTL", func(do *Do) {
			rec := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/default:ttl", map[string]any{"value": "x"}).
				Status(Is(200)).
				Capture(&rec).
				Run()
			Equal(rec.DurationSecs, uint64(300), "Omitted duration_secs should get the default TTL.")
		}).

		// 13
		Test("Omitted Duration Reuses Prior TTL", func(do *Do) {
			req := valueToCreateRecord("temp-value1", 600) // 10-minute expiration
			rec1 := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/specific:ttl", req).
				Status(Is(200)).
				Capture(&rec1).
				Run()
			rec2 := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/specific:ttl", map[string]any{"value": "temp-value2", "version": rec1.Version}).
				Status(Is(200)).
				Capture(&rec2).
				Run()
			Equal(rec2.DurationSecs, uint64(600), "Omitted duration_secs should reuse the prior TTL.")
			Equal(rec2.Value, "temp-value2", "Value for specific:ttl should match the updated value.")
			NotEqual(rec2.Version, rec1.Version, "Version for specific:ttl should change after update.")
		}).
		// 14
		Test("Can Omit Value and TTL to Renew", func(do *Do) {
			req1 := valueToCreateRecord("temp-value1", 600) // 10-minute expiration
			rec1 := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/specific:ttl2", req1).
				Status(Is(200)).
				Hint("ttl2 should be initialized nonempty.").
				Capture(&rec1).
				Run()
			req2 := valueToUpdateRecord("temp-value2", rec1.Version, 600)
			rec2 := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/specific:ttl2", req2).
				Status(Is(200)).
				Hint("ttl2 should be updated with new value and TTL.").
				Capture(&rec2).
				Run()
			Equal(rec2.DurationSecs, uint64(600), "DurationSecs for specific:ttl2 should match the updated duration.")
			Equal(rec2.Value, "temp-value2", "Value for specific:ttl2 should match the updated value.")
			NotEqual(rec2.Version, rec1.Version, "Version for specific:ttl2 should change after update.")

			time.Sleep(2 * time.Second) // expiration_secs changes each 1s
			do.PutJSON(Node("n1"), "/kv/specific:ttl2", map[string]any{"version": rec1.Version}).
				Status(Is(409)).
				Hint("ttl2 needs matching version to renew.").
				Run()
			rec3 := KeyValueRecord{}
			do.PutJSON(Node("n1"), "/kv/specific:ttl2", map[string]any{"version": rec2.Version}).
				Status(Is(200)).
				Hint("ttl2 should be renewed when correct version is provided.").
				Capture(&rec3).
				Run()
			Equal(rec3.DurationSecs, uint64(600), "Omitted duration_secs should reuse the prior TTL.")
			Equal(rec3.Value, "temp-value2", "Value for specific:ttl2 should match the initial value.")
			Equal(rec3.Version, rec2.Version, "Version for specific:ttl2 should remain the same after renewal.")
			Equal(rec3.ExpirationSecs > rec2.ExpirationSecs, true, "ExpirationSecs for specific:ttl2 should be extended after renewal.")
		})
}
