package database_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"go-project/src/database"
	"gorm.io/datatypes"
	"testing"
	"time"
)

func TestConnect(t *testing.T) {
	database.Connect(":memory:")

	// It should create all tables
	var tables []string
	database.DB.Raw(`SELECT name FROM sqlite_schema WHERE type = 'table' ORDER BY name`).Scan(&tables)
	assert.Equal(t, []string{"requests", "socket_clients"}, tables)
}

func TestCountRequests(t *testing.T) {
	database.Connect(":memory:")

	// It should return 0 if no items exist
	assert.Equal(t, int64(0), database.CountRequests())

	// It should return the amount of existing items
	var n = 3
	for i := 0; i < n; i++ {
		ID := fmt.Sprint(i)
		database.CreateRequest(&database.Request{
			UUID:       ID,
			EndpointID: ID,
			IP:         ID,
			Method:     "GET",
			Path:       "/test",
			Body:       "test",
		})
	}
	assert.Equal(t, int64(n), database.CountRequests())
}

func TestGetRequestsForEndpointID(t *testing.T) {
	database.Connect(":memory:")

	endpointID := "test-id"

	var items []database.Request

	// It should return an empty array if no items exist
	items = database.GetRequestsForEndpointID(endpointID, "", 32)
	assert.Equal(t, []database.Request{}, items)

	database.CreateRequest(&database.Request{
		UUID:       "uuid-1",
		EndpointID: endpointID,
		IP:         "test-ip",
		Method:     "GET",
		Path:       "/test",
		Body:       "test-body-1",
		Headers:    datatypes.JSON(`{ "Test": "Test-Header-1" }`),
	})
	database.CreateRequest(&database.Request{
		UUID:       "uuid-2",
		EndpointID: endpointID,
		IP:         "test-ip",
		Method:     "GET",
		Path:       "/test",
		Body:       "test-body-2",
		Headers:    datatypes.JSON(`{ "Test": "Test-Header-2" }`),
	})
	database.CreateRequest(&database.Request{
		UUID:       "uuid-3",
		EndpointID: "other-id",
		IP:         "test-ip",
		Method:     "GET",
		Path:       "/test",
		Body:       "test-body-3",
		Headers:    datatypes.JSON(`{ "Test": "Test-Header-3" }`),
	})

	// It should return items with the correct shape
	items = database.GetRequestsForEndpointID(endpointID, "", 1)
	assert.Equal(t, "uuid-2", items[0].UUID)
	assert.Equal(t, endpointID, items[0].EndpointID)
	assert.Equal(t, "test-ip", items[0].IP)
	assert.Equal(t, "GET", items[0].Method)
	assert.Equal(t, "/test", items[0].Path)
	assert.Equal(t, "test-body-2", items[0].Body)
	assert.Equal(t, datatypes.JSON(`{ "Test": "Test-Header-2" }`), items[0].Headers)
	assert.Equal(t, time.Now().Format(time.ANSIC), items[0].CreatedAt.Format(time.ANSIC))

	// It should only return items with the specified endpoint id
	items = database.GetRequestsForEndpointID(endpointID, "", 32)
	assert.Equal(t, 2, len(items))

	// It should not return more items than the limit
	items = database.GetRequestsForEndpointID(endpointID, "", 1)
	assert.Equal(t, 1, len(items))

	// It should return return items ordered by creation date, newest first
	items = database.GetRequestsForEndpointID(endpointID, "", 32)
	assert.Equal(t, "test-body-2", items[0].Body)
	assert.Equal(t, "test-body-1", items[1].Body)

	// It should not apply any additional filtering if the search string is empty
	items = database.GetRequestsForEndpointID(endpointID, "", 32)
	assert.Equal(t, 2, len(items))

	// It should search the body based on the search string
	items = database.GetRequestsForEndpointID(endpointID, "test-body", 32)
	assert.Equal(t, 2, len(items))

	items = database.GetRequestsForEndpointID(endpointID, "test-body-1", 32)
	assert.Equal(t, 1, len(items))

	// It should search the headers based on the search string
	items = database.GetRequestsForEndpointID(endpointID, "Test-Header", 32)
	assert.Equal(t, 2, len(items))

	items = database.GetRequestsForEndpointID(endpointID, "Test-Header-1", 32)
	assert.Equal(t, 1, len(items))
}

func TestCreateRequest(t *testing.T) {
	database.Connect(":memory:")

	endpointID := "test-id"

	database.CreateRequest(&database.Request{
		UUID:       "test-uuid",
		EndpointID: endpointID,
		IP:         "test-ip",
		Method:     "GET",
		Path:       "/test",
		Body:       "test-body",
		Headers:    datatypes.JSON(`{ "Test": "Test-Header" }`),
	})

	items := database.GetRequestsForEndpointID(endpointID, "", 1)

	assert.Equal(t, "test-uuid", items[0].UUID)
	assert.Equal(t, time.Now().Format(time.ANSIC), items[0].CreatedAt.Format(time.ANSIC))
}

func TestDeleteOldRequests(t *testing.T) {
	database.Connect(":memory:")

	endpointID := "test-id"

	threshold := time.Now().Add(-1 * 4 * time.Hour)

	// It should delete items created before the threshold
	database.CreateRequest(&database.Request{
		UUID:       "uuid-delete",
		EndpointID: endpointID,
		IP:         "test-ip",
		Method:     "GET",
		Path:       "/test",
		Body:       "test-body",
		Headers:    datatypes.JSON(`{ "Test": "Test-Header" }`),
		CreatedAt:  threshold.Add(-1 * time.Hour),
	})
	database.DeleteOldRequests(threshold)
	assert.Equal(t, int64(0), database.CountRequests())

	// It should not delete items created after the threshold
	database.CreateRequest(&database.Request{
		UUID:       "uuid-keep",
		EndpointID: endpointID,
		IP:         "test-ip",
		Method:     "GET",
		Path:       "/test",
		Body:       "test-body",
		Headers:    datatypes.JSON(`{ "Test": "Test-Header" }`),
		CreatedAt:  threshold.Add(1 * time.Hour),
	})
	database.DeleteOldRequests(threshold)
	assert.Equal(t, int64(1), database.CountRequests())
}
