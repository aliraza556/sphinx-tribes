package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stakwork/sphinx-tribes/logger"
	"github.com/stakwork/sphinx-tribes/utils"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/stakwork/sphinx-tribes/auth"
	"github.com/stakwork/sphinx-tribes/config"
	"github.com/stakwork/sphinx-tribes/db"
	"github.com/stakwork/sphinx-tribes/handlers/mocks"
	dbMocks "github.com/stakwork/sphinx-tribes/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var bountyOwner = db.Person{
	Uuid:        "user_3_uuid",
	OwnerAlias:  "user3",
	UniqueName:  "user3",
	OwnerPubKey: "user_3_pubkey",
	PriceToMeet: 0,
	Description: "this is test user 3",
}

var bountyAssignee = db.Person{
	Uuid:        "user_4_uuid",
	OwnerAlias:  "user4",
	UniqueName:  "user4",
	OwnerPubKey: "user_4_pubkey",
	PriceToMeet: 0,
	Description: "this is user 4",
}

var bountyPrev = db.NewBounty{
	Type:          "coding",
	Title:         "Previous bounty",
	Description:   "Previous bounty description",
	OrgUuid:       "org-4",
	WorkspaceUuid: "work-4",
	Assignee:      bountyAssignee.OwnerPubKey,
	OwnerID:       bountyOwner.OwnerPubKey,
	Show:          true,
	Created:       111111111,
}

var bountyNext = db.NewBounty{
	Type:          "coding",
	Title:         "Next bounty",
	Description:   "Next bounty description",
	WorkspaceUuid: "work-4",
	Assignee:      "",
	OwnerID:       bountyOwner.OwnerPubKey,
	Show:          true,
	Created:       111111112,
}

var workspace = db.Workspace{
	Uuid:        "workspace_uuid13",
	Name:        "TestWorkspace",
	Description: "This is a test workspace",
	OwnerPubKey: bountyOwner.OwnerPubKey,
	Img:         "",
	Website:     "",
}

var workBountyPrev = db.NewBounty{
	Type:          "coding",
	Title:         "Workspace Previous bounty",
	Description:   "Workspace Previous bounty description",
	WorkspaceUuid: workspace.Uuid,
	Assignee:      bountyAssignee.OwnerPubKey,
	OwnerID:       bountyOwner.OwnerPubKey,
	Show:          true,
	Created:       111111113,
}

var workBountyNext = db.NewBounty{
	Type:          "coding",
	Title:         "Workpace Next bounty",
	Description:   "Workspace Next bounty description",
	WorkspaceUuid: workspace.Uuid,
	Assignee:      "",
	OwnerID:       bountyOwner.OwnerPubKey,
	Show:          true,
	Created:       111111114,
}

func SetupSuite(_ *testing.T) func(tb testing.TB) {
	db.InitTestDB()

	return func(_ testing.TB) {
		defer db.CloseTestDB()
		log.Println("Teardown test")
	}
}

func AddExisitingDB(existingBounty db.NewBounty) {
	bounty := db.TestDB.GetBounty(1)
	if bounty.ID == 0 {
		// add existing bounty to db
		db.TestDB.CreateOrEditBounty(existingBounty)
	}
}

func TestCreateOrEditBounty(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	// create user
	db.TestDB.CreateOrEditPerson(bountyOwner)

	existingBounty := db.NewBounty{
		Type:          "coding",
		Title:         "existing bounty",
		Description:   "existing bounty description",
		WorkspaceUuid: "work-1",
		OwnerID:       bountyOwner.OwnerPubKey,
		Price:         2000,
	}

	// Add initial Bounty
	AddExisitingDB(existingBounty)

	newBounty := db.NewBounty{
		Type:          "coding",
		Title:         "new bounty",
		Description:   "new bounty description",
		WorkspaceUuid: "work-1",
		OwnerID:       bountyOwner.OwnerPubKey,
		Price:         1500,
	}

	failedBounty := db.NewBounty{
		Title:         "new bounty",
		Description:   "failed bounty description",
		WorkspaceUuid: "work-1",
		OwnerID:       bountyOwner.OwnerPubKey,
		Price:         1500,
	}

	ctx := context.WithValue(context.Background(), auth.ContextKey, bountyOwner.OwnerPubKey)
	mockClient := mocks.NewHttpClient(t)
	mockUserHasManageBountyRolesTrue := func(pubKeyFromAuth string, uuid string) bool {
		return true
	}
	mockUserHasManageBountyRolesFalse := func(pubKeyFromAuth string, uuid string) bool {
		return false
	}
	bHandler := NewBountyHandler(mockClient, db.TestDB)

	t.Run("should return error if body is not a valid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)

		invalidJson := []byte(`{"key": "value"`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(invalidJson))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotAcceptable, rr.Code, "invalid status received")
	})

	t.Run("missing required field, bounty type", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)

		invalidBody := []byte(`{"type": ""}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(invalidBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("missing required field, bounty title", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)

		invalidBody := []byte(`{"type": "bounty_type", "title": ""}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(invalidBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("missing required field, bounty description", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)

		invalidBody := []byte(`{"type": "bounty_type", "title": "first bounty", "description": ""}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(invalidBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("return error if trying to update other user's bounty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)
		bHandler.userHasManageBountyRoles = mockUserHasManageBountyRolesFalse

		updatedBounty := existingBounty
		updatedBounty.ID = 1
		updatedBounty.Show = true
		updatedBounty.WorkspaceUuid = ""

		json, err := json.Marshal(updatedBounty)
		if err != nil {
			logger.Log.Error("Could not marshal json data")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(json))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, strings.TrimRight(rr.Body.String(), "\n"), "Cannot edit another user's bounty")
	})

	t.Run("return error if user does not have required roles", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)
		bHandler.userHasManageBountyRoles = mockUserHasManageBountyRolesFalse

		updatedBounty := existingBounty
		updatedBounty.Title = "Existing bounty updated"
		updatedBounty.ID = 1

		body, _ := json.Marshal(updatedBounty)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should allow to add or edit bounty if user has role", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)
		bHandler.userHasManageBountyRoles = mockUserHasManageBountyRolesTrue

		updatedBounty := existingBounty
		updatedBounty.Title = "first bounty updated"
		updatedBounty.ID = 1

		body, _ := json.Marshal(updatedBounty)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		bounty := db.TestDB.GetBounty(1)
		assert.Equal(t, bounty.Title, updatedBounty.Title)
	})

	t.Run("should not update created at when bounty is updated", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)
		bHandler.userHasManageBountyRoles = mockUserHasManageBountyRolesTrue

		updatedBounty := existingBounty
		updatedBounty.Title = "second bounty updated"

		body, _ := json.Marshal(updatedBounty)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var returnedBounty db.Bounty
		err = json.Unmarshal(rr.Body.Bytes(), &returnedBounty)
		assert.NoError(t, err)
		assert.NotEqual(t, returnedBounty.Created, returnedBounty.Updated)
		// Check the response body or any other expected behavior
	})

	t.Run("should return error if failed to add new bounty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)
		body, _ := json.Marshal(failedBounty)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("add bounty if error not present", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.CreateOrEditBounty)

		body, _ := json.Marshal(newBounty)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestPayLightningInvoice(t *testing.T) {
	botURL := os.Getenv("V2_BOT_URL")
	botToken := os.Getenv("V2_BOT_TOKEN")

	expectedUrl := fmt.Sprintf("%s/invoices", config.RelayUrl)
	expectedBody := `{"payment_request": "req-id"}`

	expectedV2Url := fmt.Sprintf("%s/pay_invoice", botURL)
	expectedV2Body := `{"bolt11": "req-id", "wait": true}`

	t.Run("validate request url, body and headers", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}
		mockDb := &dbMocks.Database{}
		handler := NewBountyHandler(mockHttpClient, mockDb)

		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken && expectedV2Body == string(bodyByt)
			})).Return(nil, errors.New("some-error")).Once()
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPut && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey && expectedBody == string(bodyByt)
			})).Return(nil, errors.New("some-error")).Once()
		}

		success, invoicePayErr := handler.PayLightningInvoice("req-id")

		assert.Empty(t, invoicePayErr)
		assert.Empty(t, success)
		mockHttpClient.AssertExpectations(t)
	})

	t.Run("put on invoice request failed with error status and invalid json", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}
		mockDb := &dbMocks.Database{}
		handler := NewBountyHandler(mockHttpClient, mockDb)
		r := io.NopCloser(bytes.NewReader([]byte(`"internal server error"`)))

		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken && expectedV2Body == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil)
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPut && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey && expectedBody == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil)
		}

		success, invoicePayErr := handler.PayLightningInvoice("req-id")

		assert.False(t, invoicePayErr.Success)
		assert.Empty(t, success)
		mockHttpClient.AssertExpectations(t)
	})

	t.Run("put on invoice request failed with error status", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}
		mockDb := &dbMocks.Database{}
		handler := NewBountyHandler(mockHttpClient, mockDb)

		r := io.NopCloser(bytes.NewReader([]byte(`{"error": "internal server error"}`)))

		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken && expectedV2Body == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil)
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPut && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey && expectedBody == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil).Once()
		}
		success, invoicePayErr := handler.PayLightningInvoice("req-id")

		assert.Equal(t, invoicePayErr.Error, "internal server error")
		assert.Empty(t, success)
		mockHttpClient.AssertExpectations(t)
	})

	t.Run("put on invoice request succeed with invalid json", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}
		mockDb := &dbMocks.Database{}
		handler := NewBountyHandler(mockHttpClient, mockDb)
		r := io.NopCloser(bytes.NewReader([]byte(`"invalid json"`)))

		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken && expectedV2Body == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil)
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPut && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey && expectedBody == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       r,
			}, nil).Once()
		}

		success, invoicePayErr := handler.PayLightningInvoice("req-id")

		assert.False(t, success.Success)
		assert.Empty(t, invoicePayErr)
		mockHttpClient.AssertExpectations(t)
	})

	t.Run("should unmarshal the response properly after success", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}
		mockDb := &dbMocks.Database{}
		handler := NewBountyHandler(mockHttpClient, mockDb)

		r := io.NopCloser(bytes.NewReader([]byte(`{"success": true, "response": { "settled": true, "payment_request": "req", "payment_hash": "hash", "preimage": "random-string", "amount": "1000"}}`)))

		rv3 := io.NopCloser(bytes.NewReader([]byte(`{"status": "COMPLETE", "amt_msat": "1000", "timestamp": "" }`)))

		expectedSuccessMsg := db.InvoicePaySuccess{
			Success: true,
			Response: db.InvoiceCheckResponse{
				Settled:         true,
				Payment_request: "req",
				Payment_hash:    "hash",
				Preimage:        "random-string",
				Amount:          "1000",
			},
		}

		expectedV2SuccessMsg := db.InvoicePaySuccess{
			Success: true,
			Response: db.InvoiceCheckResponse{
				Settled:         true,
				Payment_request: "req-id",
				Payment_hash:    "",
				Preimage:        "",
				Amount:          "",
			},
		}

		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken && expectedV2Body == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       rv3,
			}, nil)

			success, invoicePayErr := handler.PayLightningInvoice("req-id")

			assert.Empty(t, invoicePayErr)
			assert.EqualValues(t, expectedV2SuccessMsg, success)
			mockHttpClient.AssertExpectations(t)
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPut && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey && expectedBody == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       r,
			}, nil).Once()

			success, invoicePayErr := handler.PayLightningInvoice("req")

			assert.Empty(t, invoicePayErr)
			assert.EqualValues(t, expectedSuccessMsg, success)
			mockHttpClient.AssertExpectations(t)
		}
	})

}

func TestDeleteBounty(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	existingBounty := db.NewBounty{
		Type:          "coding",
		Title:         "existing bounty",
		Description:   "existing bounty description",
		WorkspaceUuid: "work-1",
		OwnerID:       "first-user",
		Price:         2000,
	}

	// Add initial Bounty
	AddExisitingDB(existingBounty)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)
	ctx := context.WithValue(context.Background(), auth.ContextKey, "test-key")

	t.Run("should return unauthorized error if users public key not present", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBounty)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/", nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return unauthorized error if public key not present in route", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBounty)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("pubkey", "")
		rctx.URLParams.Add("created", "1111")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodDelete, "//1111", nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return unauthorized error if created at key not present in route", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBounty)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("pubkey", "pub-key")
		rctx.URLParams.Add("created", "")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodDelete, "/pub-key/", nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return error if failed to delete from db", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBounty)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("pubkey", "pub-key")
		rctx.URLParams.Add("created", "1111")

		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodDelete, "/pub-key/createdAt", nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("should successfully delete bounty from db", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBounty)
		existingBounty := db.TestDB.GetBounty(1)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("pubkey", existingBounty.OwnerID)

		created := fmt.Sprintf("%d", existingBounty.Created)
		rctx.URLParams.Add("created", created)

		route := fmt.Sprintf("/%s/%d", existingBounty.OwnerID, existingBounty.Created)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodDelete, route, nil)

		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)

		// get Bounty from DB
		checkBounty := db.TestDB.GetBounty(1)
		// chcek that the bounty's ID is now zero
		assert.Equal(t, 0, int(checkBounty.ID))
	})
}

func TestGetBountyByCreated(t *testing.T) {
	mockDb := dbMocks.NewDatabase(t)
	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, mockDb)

	t.Run("Should return bounty by its created value", func(t *testing.T) {
		mockGenerateBountyResponse := func(bounties []db.NewBounty) []db.BountyResponse {
			var bountyResponses []db.BountyResponse

			for _, bounty := range bounties {
				owner := db.Person{
					ID: 1,
				}
				assignee := db.Person{
					ID: 1,
				}
				workspace := db.WorkspaceShort{
					Uuid: "uuid",
				}

				bountyResponse := db.BountyResponse{
					Bounty:       bounty,
					Assignee:     assignee,
					Owner:        owner,
					Organization: workspace,
					Workspace:    workspace,
				}
				bountyResponses = append(bountyResponses, bountyResponse)
			}

			return bountyResponses
		}
		bHandler.generateBountyResponse = mockGenerateBountyResponse

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyByCreated)
		bounty := db.NewBounty{
			ID:            1,
			Type:          "coding",
			Title:         "first bounty",
			Description:   "first bounty description",
			OrgUuid:       "org-1",
			WorkspaceUuid: "work-1",
			Assignee:      "user1",
			Created:       1707991475,
			OwnerID:       "owner-1",
		}
		createdStr := strconv.FormatInt(bounty.Created, 10)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("created", "1707991475")
		req, _ := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/created/1707991475", nil)
		mockDb.On("GetBountyDataByCreated", createdStr).Return([]db.NewBounty{bounty}, nil).Once()
		mockDb.On("GetPersonByPubkey", "owner-1").Return(db.Person{}).Once()
		mockDb.On("GetPersonByPubkey", "user1").Return(db.Person{}).Once()
		mockDb.On("GetWorkspaceByUuid", "work-1").Return(db.Workspace{}).Once()
		mockDb.On("GetProofsByBountyID", bounty.ID).Return([]db.ProofOfWork{}).Once()
		handler.ServeHTTP(rr, req)

		var returnedBounty []db.BountyResponse
		err := json.Unmarshal(rr.Body.Bytes(), &returnedBounty)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, returnedBounty)

	})
	t.Run("Should return 404 if bounty is not present in db", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyByCreated)
		createdStr := ""

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("created", createdStr)
		req, _ := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/created/"+createdStr, nil)

		mockDb.On("GetBountyDataByCreated", createdStr).Return([]db.NewBounty{}, nil).Once()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code, "Expected 404 Not Found for nonexistent bounty")

		mockDb.AssertExpectations(t)
	})

}

func TestGetPersonAssignedBounties(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)
	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	bountyOwner := db.Person{
		Uuid:        "user_1_uuid",
		OwnerAlias:  "user1",
		UniqueName:  "user1",
		OwnerPubKey: "user_1_pubkey",
		PriceToMeet: 0,
		Description: "this is test user 1",
	}

	bountyAssignee := db.Person{
		Uuid:        "user_2_uuid",
		OwnerAlias:  "user2",
		UniqueName:  "user2",
		OwnerPubKey: "user_2_pubkey",
		PriceToMeet: 0,
		Description: "this is user 2",
	}

	bounty := db.NewBounty{
		Type:          "coding",
		Title:         "first bounty",
		Description:   "first bounty description",
		OrgUuid:       "org-1",
		WorkspaceUuid: "work-1",
		Assignee:      bountyAssignee.OwnerPubKey,
		OwnerID:       bountyOwner.OwnerPubKey,
		Show:          true,
	}

	t.Run("Should successfull Get Person Assigned Bounties", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetPersonAssignedBounties)

		// create users
		db.TestDB.CreateOrEditPerson(bountyOwner)
		db.TestDB.CreateOrEditPerson(bountyAssignee)

		// create bounty
		db.TestDB.CreateOrEditBounty(bounty)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", bountyAssignee.Uuid)
		rctx.URLParams.Add("sortBy", "paid")
		rctx.URLParams.Add("page", "0")
		rctx.URLParams.Add("limit", "20")
		rctx.URLParams.Add("search", "")

		route := fmt.Sprintf("/people/wanteds/assigned/%s?sortBy=paid&page=0&limit=20&search=''", bountyAssignee.Uuid)
		req, _ := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, route, nil)

		handler.ServeHTTP(rr, req)

		// bounty from db
		expectedBounty, _ := db.TestDB.GetAssignedBounties(req)

		var returnedBounty []db.BountyResponse
		err := json.Unmarshal(rr.Body.Bytes(), &returnedBounty)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, returnedBounty)
		assert.Equal(t, len(expectedBounty), len(returnedBounty))
	})
}

func TestGetPersonCreatedBounties(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	ctx := context.Background()
	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	bounty := db.NewBounty{
		Type:          "coding",
		Title:         "first bounty 3",
		Description:   "first bounty description",
		WorkspaceUuid: "work-4",
		Assignee:      bountyAssignee.OwnerPubKey,
		OwnerID:       bountyOwner.OwnerPubKey,
		Show:          true,
	}

	bounty2 := db.NewBounty{
		Type:          "coding 2",
		Title:         "second bounty 3",
		Description:   "second bounty description 2",
		OrgUuid:       "org-4",
		WorkspaceUuid: "work-4",
		Assignee:      bountyAssignee.OwnerPubKey,
		OwnerID:       bountyOwner.OwnerPubKey,
		Show:          true,
		Created:       11111111,
	}

	bounty3 := db.NewBounty{
		Type:          "coding 2",
		Title:         "second bounty 4",
		Description:   "second bounty description 2",
		WorkspaceUuid: "work-4",
		Assignee:      "",
		OwnerID:       bountyOwner.OwnerPubKey,
		Show:          true,
		Created:       2222222,
	}

	// create users
	db.TestDB.CreateOrEditPerson(bountyOwner)
	db.TestDB.CreateOrEditPerson(bountyAssignee)

	// create bounty
	db.TestDB.CreateOrEditBounty(bounty)
	db.TestDB.CreateOrEditBounty(bounty2)
	db.TestDB.CreateOrEditBounty(bounty3)

	t.Run("should return bounties created by the user", func(t *testing.T) {
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", bountyOwner.Uuid)

		route := fmt.Sprintf("/people/wanteds/created/%s?sortBy=paid&page=1&limit=20&search=''", bountyOwner.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, route, nil)
		if err != nil {
			t.Fatal(err)
		}

		bHandler.GetPersonCreatedBounties(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var responseData []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &responseData)
		if err != nil {
			t.Fatalf("Error decoding JSON response: %s", err)
		}

		// bounty from db
		expectedBounty, _ := db.TestDB.GetCreatedBounties(req)

		assert.NotEmpty(t, responseData)
		assert.Equal(t, len(expectedBounty), len(responseData))
	})

	t.Run("should not return bounties created by other users", func(t *testing.T) {
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", bountyAssignee.Uuid)

		route := fmt.Sprintf("/people/wanteds/created/%s?sortBy=paid&page=1&limit=20&search=''", bountyAssignee.Uuid)
		req, err := http.NewRequest("GET", route, nil)
		req = req.WithContext(ctx)
		if err != nil {
			t.Fatal(err)
		}

		bHandler.GetPersonCreatedBounties(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var responseData []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &responseData)
		if err != nil {
			t.Fatalf("Error decoding JSON response: %s", err)
		}

		assert.Empty(t, responseData)
		assert.Len(t, responseData, 0)
	})

	t.Run("should filter bounties by status and apply pagination", func(t *testing.T) {
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", bountyOwner.Uuid)

		route := fmt.Sprintf("/people/wanteds/created/%s?Assigned=true&page=1&limit=2", bountyOwner.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, route, nil)
		if err != nil {
			t.Fatal(err)
		}

		bHandler.GetPersonCreatedBounties(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var responseData []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &responseData)
		if err != nil {
			t.Fatalf("Error decoding JSON response: %s", err)
		}

		assert.Len(t, responseData, 2)

		// Assert that bounties are filtered correctly
		// bounty from db
		expectedBounty, _ := db.TestDB.GetCreatedBounties(req)
		assert.Equal(t, len(expectedBounty), len(responseData))
	})
}

func TestGetNextBountyByCreated(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	db.TestDB.CreateOrEditBounty(bountyPrev)
	db.TestDB.CreateOrEditBounty(bountyNext)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	t.Run("Should test that the next bounty on the bounties homepage can be gotten by its created value and the selected filters", func(t *testing.T) {
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		created := fmt.Sprintf("%d", bountyPrev.Created)
		rctx.URLParams.Add("created", created)

		route := fmt.Sprintf("/next/%d", bountyPrev.Created)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, route, nil)
		if err != nil {
			t.Fatal(err)
		}
		bHandler.GetNextBountyByCreated(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var responseData uint
		err = json.Unmarshal(rr.Body.Bytes(), &responseData)
		if err != nil {
			t.Fatalf("Error decoding JSON response: %s", err)
		}
		assert.Greater(t, responseData, uint(1))
	})
}

func TestGetPreviousBountyByCreated(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	t.Run("Should test that the previous bounty on the bounties homepage can be gotten by its created value and the selected filters", func(t *testing.T) {
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		created := fmt.Sprintf("%d", bountyPrev.Created)
		rctx.URLParams.Add("created", created)

		route := fmt.Sprintf("/previous/%d", bountyNext.Created)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, route, nil)
		if err != nil {
			t.Fatal(err)
		}

		bHandler.GetPreviousBountyByCreated(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var responseData uint
		err = json.Unmarshal(rr.Body.Bytes(), &responseData)
		if err != nil {
			t.Fatalf("Error decoding JSON response: %s", err)
		}
		assert.Greater(t, responseData, uint(1))
	})
}

func TestGetWorkspaceNextBountyByCreated(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	db.TestDB.CreateOrEditWorkspace(workspace)
	db.TestDB.CreateOrEditBounty(workBountyPrev)
	db.TestDB.CreateOrEditBounty(workBountyNext)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	t.Run("Should test that the next bounty on the workspace bounties homepage can be gotten by its created value and the selected filters", func(t *testing.T) {
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		created := fmt.Sprintf("%d", workBountyPrev.Created)
		rctx.URLParams.Add("created", created)
		rctx.URLParams.Add("uuid", workspace.Uuid)

		route := fmt.Sprintf("/org/next/%s/%d", workspace.Uuid, workBountyPrev.Created)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, route, nil)
		if err != nil {
			t.Fatal(err)
		}

		bHandler.GetWorkspaceNextBountyByCreated(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var responseData uint
		err = json.Unmarshal(rr.Body.Bytes(), &responseData)
		if err != nil {
			t.Fatalf("Error decoding JSON response: %s", err)
		}
		assert.Greater(t, responseData, uint(2))
	})
}

func TestGetWorkspacePreviousBountyByCreated(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	db.TestDB.CreateOrEditWorkspace(workspace)
	db.TestDB.CreateOrEditBounty(workBountyPrev)
	db.TestDB.CreateOrEditBounty(workBountyNext)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	t.Run("Should test that the previous bounty on the workspace bounties homepage can be gotten by its created value and the selected filters", func(t *testing.T) {
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		created := fmt.Sprintf("%d", workBountyNext.Created)
		rctx.URLParams.Add("created", created)
		rctx.URLParams.Add("uuid", workspace.Uuid)

		route := fmt.Sprintf("/org/previous/%s/%d", workspace.Uuid, workBountyNext.Created)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, route, nil)
		if err != nil {
			t.Fatal(err)
		}

		bHandler.GetWorkspacePreviousBountyByCreated(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var responseData uint
		err = json.Unmarshal(rr.Body.Bytes(), &responseData)
		if err != nil {
			t.Fatalf("Error decoding JSON response: %s", err)
		}
		assert.Greater(t, responseData, uint(0))
	})
}

func TestGetBountyById(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	t.Run("successful retrieval of bounty by ID", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyById)

		now := time.Now().Unix()
		bounty := db.NewBounty{
			Type:          "coding",
			Title:         "Bounty With ID",
			Description:   "Bounty ID description",
			WorkspaceUuid: "",
			Assignee:      "",
			OwnerID:       bountyOwner.OwnerPubKey,
			Show:          true,
			Created:       now,
		}

		db.TestDB.CreateOrEditBounty(bounty)

		bountyInDb, err := db.TestDB.GetBountyByCreated(uint(bounty.Created))
		assert.NoError(t, err)
		assert.NotNil(t, bountyInDb)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("bountyId", strconv.Itoa(int(bountyInDb.ID)))
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/bounty/"+strconv.Itoa(int(bountyInDb.ID)), nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var returnedBounty []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &returnedBounty)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, returnedBounty)
	})

	t.Run("bounty not found", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyById)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("bountyId", "Invalid-id")
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/bounty/Invalid-id", nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestGetBountyIndexById(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	db.DeleteAllBounties()

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	t.Run("successful retrieval of bounty by Index ID", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyIndexById)

		now := time.Now().UnixMilli()
		bounty := db.NewBounty{
			ID:            1,
			Type:          "coding",
			Title:         "Bounty With ID",
			Description:   "Bounty description",
			WorkspaceUuid: "",
			Assignee:      "",
			OwnerID:       bountyOwner.OwnerPubKey,
			Show:          true,
			Created:       now,
			MaxStakers:    1,
		}

		db.TestDB.CreateOrEditBounty(bounty)

		bountyInDb, err := db.TestDB.GetBountyByCreated(uint(bounty.Created))
		assert.NoError(t, err)
		assert.Equal(t, bounty.ID, bountyInDb.ID)
		assert.Equal(t, bounty.Title, bountyInDb.Title)
		assert.Equal(t, bounty.Created, bountyInDb.Created)

		bountyIndex := db.TestDB.GetBountyIndexById(strconv.Itoa(int(bountyInDb.ID)))

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("bountyId", strconv.Itoa(int(bountyInDb.ID)))
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/index/"+strconv.Itoa(int(bountyInDb.ID)), nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		responseBody := rr.Body.Bytes()
		responseString := strings.TrimSpace(string(responseBody))
		returnedIndex, err := strconv.ParseInt(responseString, 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, bountyIndex, returnedIndex)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("bounty index by ID not found", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyIndexById)

		bountyID := ""
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("bountyId", bountyID)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/index/"+bountyID, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestGetAllBounties(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	t.Run("Should successfully return all bounties", func(t *testing.T) {
		now := time.Now().Unix()
		bounty := db.NewBounty{
			Type:          "coding",
			Title:         "Bounty With ID",
			Description:   "Bounty ID description",
			WorkspaceUuid: "",
			Assignee:      "",
			OwnerID:       "test-owner",
			Show:          true,
			Created:       now,
		}
		db.TestDB.CreateOrEditBounty(bounty)

		bountyInDb, err := db.TestDB.GetBountyByCreated(uint(bounty.Created))
		assert.NoError(t, err)
		assert.NotNil(t, bountyInDb)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetAllBounties)

		rctx := chi.NewRouteContext()
		req, _ := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/all", nil)

		handler.ServeHTTP(rr, req)

		var returnedBounty []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &returnedBounty)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, returnedBounty)
	})
}

func MockNewWSServer(t *testing.T) (*httptest.Server, *websocket.Conn) {

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{}

		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Log.Error("upgrade error: %v", err)
			return
		}
		defer ws.Close()
	}))
	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	return s, ws
}

func TestMakeBountyPayment(t *testing.T) {
	ctx := context.Background()

	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := &mocks.HttpClient{}
	mockUserHasAccessTrue := func(pubKeyFromAuth string, uuid string, role string) bool {
		return true
	}
	mockUserHasAccessFalse := func(pubKeyFromAuth string, uuid string, role string) bool {
		return false
	}
	mockGetSocketConnections := func(host string) (db.Client, error) {
		s, ws := MockNewWSServer(t)
		defer s.Close()
		defer ws.Close()

		mockClient := db.Client{
			Host: "mocked_host",
			Conn: ws,
		}

		return mockClient, nil
	}
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	var mutex sync.Mutex
	var processingTimes []time.Time

	now := time.Now().UnixMilli()
	bountyOwnerId := "owner_pubkey"

	botURL := os.Getenv("V2_BOT_URL")
	botToken := os.Getenv("V2_BOT_TOKEN")

	person := db.Person{
		Uuid:           "uuid",
		OwnerAlias:     "alias",
		UniqueName:     "unique_name",
		OwnerPubKey:    "03b2205df68d90f8f9913650bc3161761b61d743e615a9faa7ffecea3380a93fc1",
		OwnerRouteHint: "02162c52716637fb8120ab0261e410b185d268d768cc6f6227c58102d194ad0bc2_1099607703554",
		PriceToMeet:    0,
		Description:    "description",
	}

	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        "workspace_uuid",
		Name:        "workspace_name",
		OwnerPubKey: person.OwnerPubKey,
		Github:      "gtihub",
		Website:     "website",
		Description: "description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	budgetAmount := uint(5000)
	bountyBudget := db.NewBountyBudget{
		WorkspaceUuid: workspace.Uuid,
		TotalBudget:   budgetAmount,
	}
	db.TestDB.CreateWorkspaceBudget(bountyBudget)

	bountyAmount := uint(3000)
	bounty := db.NewBounty{
		OwnerID:       bountyOwnerId,
		Price:         bountyAmount,
		Created:       now,
		Type:          "coding",
		Title:         "bountyTitle",
		Description:   "bountyDescription",
		Assignee:      person.OwnerPubKey,
		Show:          true,
		WorkspaceUuid: workspace.Uuid,
		Paid:          false,
	}
	db.TestDB.CreateOrEditBounty(bounty)

	dbBounty, err := db.TestDB.GetBountyDataByCreated(strconv.FormatInt(bounty.Created, 10))
	if err != nil {
		t.Fatal(err)
	}

	bountyId := dbBounty[0].ID
	bountyIdStr := strconv.FormatInt(int64(bountyId), 10)

	unauthorizedCtx := context.WithValue(ctx, auth.ContextKey, "")
	authorizedCtx := context.WithValue(ctx, auth.ContextKey, person.OwnerPubKey)

	t.Run("mutex lock ensures sequential access", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mutex.Lock()
			processingTimes = append(processingTimes, time.Now())
			time.Sleep(10 * time.Millisecond)
			mutex.Unlock()

			bHandler.MakeBountyPayment(w, r)
		}))
		defer server.Close()

		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := http.Get(server.URL)
				if err != nil {
					t.Errorf("Failed to send request: %v", err)
				}
			}()
		}
		wg.Wait()

		for i := 1; i < len(processingTimes); i++ {
			assert.True(t, processingTimes[i].After(processingTimes[i-1]),
				"Expected processing times to be sequential, indicating mutex is locking effectively.")
		}
	})

	t.Run("401 unauthorized error when unauthorized user hits endpoint", func(t *testing.T) {

		r := chi.NewRouter()
		r.Post("/gobounties/pay/{id}", bHandler.MakeBountyPayment)

		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/gobounties/pay/"+bountyIdStr, nil)

		if err != nil {
			t.Fatal(err)
		}

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 Unauthorized for unauthorized access")
	})

	t.Run("401 error if user not workspace admin or does not have PAY BOUNTY role", func(t *testing.T) {
		bHandler.userHasAccess = mockUserHasAccessFalse

		r := chi.NewRouter()
		r.Post("/gobounties/pay/{id}", bHandler.MakeBountyPayment)

		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/gobounties/pay/"+bountyIdStr, bytes.NewBufferString(`{}`))
		if err != nil {
			t.Fatal(err)
		}

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 Unauthorized when the user lacks the PAY BOUNTY role")

	})

	t.Run("Should test that an error WebSocket message is sent if the payment fails", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}

		bHandler2 := NewBountyHandler(mockHttpClient, db.TestDB)
		bHandler2.getSocketConnections = mockGetSocketConnections
		bHandler2.userHasAccess = mockUserHasAccessTrue

		memoData := fmt.Sprintf("Payment For: %ss", bounty.Title)
		memoText := url.QueryEscape(memoData)

		expectedUrl := fmt.Sprintf("%s/payment", config.RelayUrl)
		expectedBody := fmt.Sprintf(`{"amount": %d, "destination_key": "%s", "text": "memotext added for notification", "data": "%s"}`, bountyAmount, person.OwnerPubKey, memoText)

		expectedV2Url := fmt.Sprintf("%s/pay", botURL)
		expectedV2Body :=
			fmt.Sprintf(`{"amt_msat": %d, "dest": "%s", "route_hint": "%s", "data": "%s", "wait": true}`, bountyAmount*1000, person.OwnerPubKey, person.OwnerRouteHint, memoText)

		r := io.NopCloser(bytes.NewReader([]byte(`"internal server error"`)))
		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken && expectedV2Body == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 406,
				Body:       r,
			}, nil).Once()
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey && expectedBody == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil).Once()
		}

		ro := chi.NewRouter()
		ro.Post("/gobounties/pay/{id}", bHandler2.MakeBountyPayment)

		requestBody := bytes.NewBuffer([]byte("{}"))
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/gobounties/pay/"+bountyIdStr, requestBody)
		if err != nil {
			t.Fatal(err)
		}

		ro.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockHttpClient.AssertExpectations(t)
	})

	t.Run("Should test that a successful WebSocket message is sent if the payment is successful", func(t *testing.T) {

		bHandler.getSocketConnections = mockGetSocketConnections
		bHandler.userHasAccess = mockUserHasAccessTrue

		memoData := fmt.Sprintf("Payment For: %ss", bounty.Title)
		memoText := url.QueryEscape(memoData)

		expectedUrl := fmt.Sprintf("%s/payment", config.RelayUrl)
		expectedBody := fmt.Sprintf(`{"amount": %d, "destination_key": "%s", "text": "memotext added for notification", "data": "%s"}`, bountyAmount, person.OwnerPubKey, memoText)

		expectedV2Url := fmt.Sprintf("%s/pay", botURL)
		expectedV2Body :=
			fmt.Sprintf(`{"amt_msat": %d, "dest": "%s", "route_hint": "%s", "data": "%s", "wait": true}`, bountyAmount*1000, person.OwnerPubKey, person.OwnerRouteHint, memoText)

		if botURL != "" && botToken != "" {
			rv2 := io.NopCloser(bytes.NewReader([]byte(`{"status": "COMPLETE", "tag": "", "preimage": "", "payment_hash": "" }`)))
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken && expectedV2Body == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       rv2,
			}, nil).Once()
		} else {
			r := io.NopCloser(bytes.NewReader([]byte(`{"success": true, "response": { "sumAmount": "1"}}`)))
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				bodyByt, _ := io.ReadAll(req.Body)
				return req.Method == http.MethodPost && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey && expectedBody == string(bodyByt)
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       r,
			}, nil).Once()
		}

		ro := chi.NewRouter()
		ro.Post("/gobounties/pay/{id}", bHandler.MakeBountyPayment)

		requestBody := bytes.NewBuffer([]byte("{}"))
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/gobounties/pay/"+bountyIdStr, requestBody)
		if err != nil {
			t.Fatal(err)
		}

		ro.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		mockHttpClient.AssertExpectations(t)

		updatedBounty := db.TestDB.GetBounty(bountyId)
		assert.True(t, updatedBounty.Paid, "Expected bounty to be marked as paid")

		updatedWorkspaceBudget := db.TestDB.GetWorkspaceBudget(bounty.WorkspaceUuid)
		assert.Equal(t, budgetAmount-bountyAmount, updatedWorkspaceBudget.TotalBudget, "Expected workspace budget to be reduced by bounty amount")
	})

	t.Run("405 when trying to pay an already-paid bounty", func(t *testing.T) {
		r := chi.NewRouter()
		r.Post("/gobounties/pay/{id}", bHandler.MakeBountyPayment)

		requestBody := bytes.NewBuffer([]byte("{}"))
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/gobounties/pay/"+bountyIdStr, requestBody)
		if err != nil {
			t.Fatal(err)
		}

		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "Expected 405 Method Not Allowed for an already-paid bounty")
	})

	t.Run("403 error when amount exceeds workspace's budget balance", func(t *testing.T) {
		db.TestDB.DeleteBounty(bountyOwnerId, strconv.FormatInt(now, 10))
		bounty.Paid = false
		db.TestDB.CreateOrEditBounty(bounty)

		dbBounty, err := db.TestDB.GetBountyDataByCreated(strconv.FormatInt(bounty.Created, 10))
		if err != nil {
			t.Fatal(err)
		}

		bountyId := dbBounty[0].ID
		bountyIdStr := strconv.FormatInt(int64(bountyId), 10)

		mockHttpClient := mocks.NewHttpClient(t)
		bHandler := NewBountyHandler(mockHttpClient, db.TestDB)
		bHandler.userHasAccess = mockUserHasAccessTrue

		r := chi.NewRouter()
		r.Post("/gobounties/pay/{id}", bHandler.MakeBountyPayment)

		requestBody := bytes.NewBuffer([]byte("{}"))
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/gobounties/pay/"+bountyIdStr, requestBody)
		if err != nil {
			t.Fatal(err)
		}

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 Forbidden when the payment exceeds the workspace's budget")
	})
}

func TestUpdateBountyPaymentStatus(t *testing.T) {
	ctx := context.Background()

	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := &mocks.HttpClient{}

	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	paymentTag := "update_tag"

	mockPendingGetInvoiceStatusByTag := func(tag string) db.V2TagRes {
		return db.V2TagRes{
			Status: db.PaymentPending,
			Tag:    paymentTag,
			Error:  "",
		}

	}
	mockCompleteGetInvoiceStatusByTag := func(tag string) db.V2TagRes {
		return db.V2TagRes{
			Status: db.PaymentComplete,
			Tag:    paymentTag,
			Error:  "",
		}
	}

	now := time.Now().UnixMilli()
	bountyOwnerId := "owner_pubkey"

	person := db.Person{
		Uuid:           "update_payment_uuid",
		OwnerAlias:     "update_alias",
		UniqueName:     "update_unique_name",
		OwnerPubKey:    "03b2205df68d90f8f9913650bc3161761b61d743e615a9faa7ffecea3380a99fg1",
		OwnerRouteHint: "02162c52716637fb8120ab0261e410b185d268d768cc6f6227c58102d194ad0bc2_1088607703554",
		PriceToMeet:    0,
		Description:    "update_description",
	}

	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        "update_workspace_uuid",
		Name:        "update_workspace_name",
		OwnerPubKey: person.OwnerPubKey,
		Github:      "gtihub",
		Website:     "website",
		Description: "update_description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	budgetAmount := uint(10000)
	bountyBudget := db.NewBountyBudget{
		WorkspaceUuid: workspace.Uuid,
		TotalBudget:   budgetAmount,
	}
	db.TestDB.CreateWorkspaceBudget(bountyBudget)

	bountyAmount := uint(3000)
	bounty := db.NewBounty{
		OwnerID:       bountyOwnerId,
		Price:         bountyAmount,
		Created:       now,
		Type:          "coding",
		Title:         "updateBountyTitle",
		Description:   "updateBountyDescription",
		Assignee:      person.OwnerPubKey,
		Show:          true,
		WorkspaceUuid: workspace.Uuid,
		Paid:          false,
	}
	db.TestDB.CreateOrEditBounty(bounty)

	dbBounty, err := db.TestDB.GetBountyDataByCreated(strconv.FormatInt(bounty.Created, 10))
	if err != nil {
		t.Fatal(err)
	}

	bountyId := dbBounty[0].ID
	bountyIdStr := strconv.FormatInt(int64(bountyId), 10)

	paymentTime := time.Now()

	payment := db.NewPaymentHistory{
		BountyId:       bountyId,
		PaymentStatus:  db.PaymentPending,
		WorkspaceUuid:  workspace.Uuid,
		PaymentType:    db.Payment,
		SenderPubKey:   person.OwnerPubKey,
		ReceiverPubKey: person.OwnerPubKey,
		Tag:            paymentTag,
		Status:         true,
		Created:        &paymentTime,
		Updated:        &paymentTime,
	}

	db.TestDB.AddPaymentHistory(payment)

	unauthorizedCtx := context.WithValue(ctx, auth.ContextKey, "")
	authorizedCtx := context.WithValue(ctx, auth.ContextKey, person.OwnerPubKey)

	t.Run("401 unauthorized error when unauthorized user hits endpoint", func(t *testing.T) {

		r := chi.NewRouter()
		r.Post("/gobounties/payment/status/{id}", bHandler.UpdateBountyPaymentStatus)

		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/gobounties/payment/status/"+bountyIdStr, nil)

		if err != nil {
			t.Fatal(err)
		}

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 Unauthorized for unauthorized access")
	})

	t.Run("Should test that a PENDING payment_status is sent if the payment is not successful", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}

		bHandler := NewBountyHandler(mockHttpClient, db.TestDB)
		bHandler.getInvoiceStatusByTag = mockPendingGetInvoiceStatusByTag

		ro := chi.NewRouter()
		ro.Put("/gobounties/payment/status/{id}", bHandler.UpdateBountyPaymentStatus)

		rr := httptest.NewRecorder()
		requestBody := bytes.NewBuffer([]byte("{}"))
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPut, "/gobounties/payment/status/"+bountyIdStr, requestBody)
		if err != nil {
			t.Fatal(err)
		}

		ro.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		mockHttpClient.AssertExpectations(t)
	})

	t.Run("Should test that a COMPLETE payment_status is sent if the payment is successful", func(t *testing.T) {
		mockHttpClient := &mocks.HttpClient{}

		bHandler := NewBountyHandler(mockHttpClient, db.TestDB)
		bHandler.getInvoiceStatusByTag = mockCompleteGetInvoiceStatusByTag

		ro := chi.NewRouter()
		ro.Put("/gobounties/payment/status/{id}", bHandler.UpdateBountyPaymentStatus)

		requestBody := bytes.NewBuffer([]byte("{}"))
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPut, "/gobounties/payment/status/"+bountyIdStr, requestBody)
		if err != nil {
			t.Fatal(err)
		}

		ro.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		mockHttpClient.AssertExpectations(t)

		payment := db.TestDB.GetPaymentByBountyId(payment.BountyId)

		updatedBounty := db.TestDB.GetBounty(bountyId)
		assert.True(t, updatedBounty.Paid, "Expected bounty to be marked as paid")
		assert.Equal(t, payment.PaymentStatus, db.PaymentComplete, "Expected Payment Status To be Complete")
	})

	t.Run("405 when trying to update an already-paid bounty", func(t *testing.T) {
		r := chi.NewRouter()
		r.Put("/gobounties/payment/status/{id}", bHandler.UpdateBountyPaymentStatus)

		requestBody := bytes.NewBuffer([]byte("{}"))
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPut, "/gobounties/payment/status/"+bountyIdStr, requestBody)
		if err != nil {
			t.Fatal(err)
		}

		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "Expected 405 Method Not Allowed for an already-paid bounty")
	})
}

func TestBountyBudgetWithdraw(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	ctx := context.Background()
	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	handlerUserHasAccess := func(pubKeyFromAuth string, uuid string, role string) bool {
		return true
	}

	handlerUserNotAccess := func(pubKeyFromAuth string, uuid string, role string) bool {
		return false
	}

	getHoursDifference := func(createdDate int64, endDate *time.Time) int64 {
		return 2
	}

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-alias",
		UniqueName:  "test-unique-name",
		OwnerPubKey: "test-pubkey",
		PriceToMeet: 0,
		Description: "test-description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-workspace" + uuid.New().String(),
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	budgetAmount := uint(5000)

	paymentTime := time.Now()

	payment := db.NewPaymentHistory{
		Amount:         budgetAmount,
		WorkspaceUuid:  workspace.Uuid,
		PaymentType:    db.Deposit,
		SenderPubKey:   person.OwnerPubKey,
		ReceiverPubKey: person.OwnerPubKey,
		Tag:            "test_deposit",
		Status:         true,
		Created:        &paymentTime,
		Updated:        &paymentTime,
	}

	db.TestDB.AddPaymentHistory(payment)

	budget := db.NewBountyBudget{
		WorkspaceUuid: workspace.Uuid,
		TotalBudget:   budgetAmount,
	}
	db.TestDB.CreateWorkspaceBudget(budget)

	unauthorizedCtx := context.WithValue(context.Background(), auth.ContextKey, "")
	authorizedCtx := context.WithValue(ctx, auth.ContextKey, person.OwnerPubKey)

	t.Run("401 error if user is unauthorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.BountyBudgetWithdraw)

		req, err := http.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/budget/withdraw", nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Should test that a 406 error is returned if wrong data is passed", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.BountyBudgetWithdraw)

		invalidJson := []byte(`"key": "value"`)

		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/budget/withdraw", bytes.NewReader(invalidJson))
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotAcceptable, rr.Code)
	})

	t.Run("401 error if user is not the workspace admin or does not have WithdrawBudget role", func(t *testing.T) {
		bHandler.userHasAccess = handlerUserNotAccess

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.BountyBudgetWithdraw)

		validData := []byte(`{"workspace_uuid": "workspace-uuid", "paymentRequest": "invoice"}`)
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/budget/withdraw", bytes.NewReader(validData))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "You don't have appropriate permissions to withdraw bounty budget")
	})

	t.Run("403 error when amount exceeds workspace's budget", func(t *testing.T) {

		bHandler.userHasAccess = handlerUserHasAccess

		invoice := "lnbc100u1png0l8ypp5hna5vnd2hcskpf69rt5y9dly2p202lejcacj53md32wx87vc2mnqdqzvscqzpgxqyz5vqrzjqwnw5tv745sjpvft6e3f9w62xqk826vrm3zaev4nvj6xr3n065aukqqqqyqqpmgqqyqqqqqqqqqqqqqqqqsp5cdg0c2qhuewz4j8680pf5va0l9a382qa5sakg4uga4nv4wnuf5qs9qrssqpdddmqtflxz3553gm5xq8ptdpl2t3ew49hgjnta0v0eyz747drkkhmnk5yxg676kvmgyugm35cts9dmrnt9mcgejg64kwk9nwxqg43cqcvxm44"

		amount := utils.GetInvoiceAmount(invoice)
		assert.Equal(t, uint(10000), amount)

		withdrawRequest := db.NewWithdrawBudgetRequest{
			PaymentRequest: invoice,
			WorkspaceUuid:  workspace.Uuid,
		}
		requestBody, _ := json.Marshal(withdrawRequest)
		req, _ := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/budget/withdraw", bytes.NewReader(requestBody))

		rr := httptest.NewRecorder()

		bHandler.BountyBudgetWithdraw(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 Forbidden when the payment exceeds the workspace's budget")
		assert.Contains(t, rr.Body.String(), "Workspace budget is not enough to withdraw the amount", "Expected specific error message")
	})

	t.Run("budget invoices get paid if amount is lesser than workspace's budget", func(t *testing.T) {
		mockHttpClient := mocks.NewHttpClient(t)
		bHandler := NewBountyHandler(mockHttpClient, db.TestDB)
		bHandler.userHasAccess = handlerUserHasAccess

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.BountyBudgetWithdraw)
		paymentAmount := uint(300)
		initialBudget := budget.TotalBudget
		expectedFinalBudget := initialBudget - paymentAmount
		budget.TotalBudget = expectedFinalBudget

		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"status": "COMPLETE", "amt_msat": "1000", "timestamp": "" }`)),
		}, nil)

		invoice := "lnbc3u1pngsqv8pp5vl6ep8llmg3f9sfu8j7ctcnphylpnjduuyljqf3sc30z6ejmrunqdqzvscqzpgxqyz5vqrzjqwnw5tv745sjpvft6e3f9w62xqk826vrm3zaev4nvj6xr3n065aukqqqqyqqz9gqqyqqqqqqqqqqqqqqqqsp5n9hrrw6pr89qn3c82vvhy697wp45zdsyhm7tnu536ga77ytvxxaq9qrssqqqhenjtquz8wz5tym8v830h9gjezynjsazystzj6muhw4rd9ccc40p8sazjuk77hhcj0xn72lfyee3tsfl7lucxkx5xgtfaqya9qldcqr3072z"

		withdrawRequest := db.NewWithdrawBudgetRequest{
			PaymentRequest: invoice,
			WorkspaceUuid:  workspace.Uuid,
		}

		requestBody, _ := json.Marshal(withdrawRequest)
		req, _ := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/budget/withdraw", bytes.NewReader(requestBody))

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var response db.InvoicePaySuccess
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Success, "Expected invoice payment to succeed")

		finalBudget := db.TestDB.GetWorkspaceBudget(workspace.Uuid)
		assert.Equal(t, expectedFinalBudget, finalBudget.TotalBudget, "The workspace's final budget should reflect the deductions from the successful withdrawals")
	})

	t.Run("400 BadRequest error if there is an error with invoice payment", func(t *testing.T) {
		bHandler.getHoursDifference = getHoursDifference

		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: 400,
			Body:       io.NopCloser(bytes.NewBufferString(`{"success": false, "error": "Payment error"}`)),
		}, nil)

		invoice := "lnbcrt1u1pnv5ejzdqad9h8vmmfvdjjqen0wgsrzvpsxqcrqpp58xyhvymlhc8q05z930fknk2vdl8wnpm5zlx5lgp4ev9u8h7yd4kssp5nu652c5y0epuxeawn8szcgdrjxwk7pfkdh9tsu44r7hacg52nfgq9qrsgqcqpjxqrrssrzjqgtzc5n3vcmlhqfq4vpxreqskxzay6xhdrxx7c38ckqs95v5459uyqqqqyqq9ggqqsqqqqqqqqqqqqqq9gwyffzjpnrwt6yswwd4znt2xqnwjwxgq63qxudru95a8pqeer2r7sduurtstz5x60y4e7m4y9nx6rqy5sr9k08vtwv6s37xh0z5pdwpgqxeqdtv"

		withdrawRequest := db.NewWithdrawBudgetRequest{
			PaymentRequest: invoice,
			WorkspaceUuid:  workspace.Uuid,
		}
		requestBody, _ := json.Marshal(withdrawRequest)
		req, _ := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/budget/withdraw", bytes.NewReader(requestBody))

		rr := httptest.NewRecorder()

		bHandler.BountyBudgetWithdraw(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Payment error", response["error"].(string))
		mockHttpClient.AssertCalled(t, "Do", mock.AnythingOfType("*http.Request"))
	})

	t.Run("Should test that an Workspace's Budget Total Amount is accurate after three (3) successful 'Budget Withdrawal Requests'", func(t *testing.T) {
		paymentAmount := uint(1000)
		initialBudget := budget.TotalBudget
		invoice := "lnbcrt10u1pnv7nz6dqld9h8vmmfvdjjqen0wgsrzvpsxqcrqvqpp54v0synj4q3j2usthzt8g5umteky6d2apvgtaxd7wkepkygxgqdyssp5lhv2878qjas3azv3nnu8r6g3tlgejl7mu7cjzc9q5haygrpapd4s9qrsgqcqpjxqrrssrzjqgtzc5n3vcmlhqfq4vpxreqskxzay6xhdrxx7c38ckqs95v5459uyqqqqyqqtwsqqgqqqqqqqqqqqqqq9gea2fjj7q302ncprk2pawk4zdtayycvm0wtjpprml96h9vujvmqdp0n5z8v7lqk44mq9620jszwaevj0mws7rwd2cegxvlmfszwgpgfqp2xafjf"

		bHandler.userHasAccess = handlerUserHasAccess
		bHandler.getHoursDifference = getHoursDifference

		for i := 0; i < 3; i++ {
			expectedFinalBudget := initialBudget - (paymentAmount * uint(i+1))
			mockHttpClient.ExpectedCalls = nil
			mockHttpClient.Calls = nil

			// add a zero amount withdrawal with a time lesser than 2 + loop index hours to beat the 1 hour withdrawal timer
			dur := int(time.Hour.Hours())*2 + i + 1
			paymentTime = time.Now().Add(-time.Hour * time.Duration(dur))

			mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"status": "COMPLETE", "amt_msat": "1000", "timestamp": "" }`)),
			}, nil)

			withdrawRequest := db.NewWithdrawBudgetRequest{
				PaymentRequest: invoice,
				WorkspaceUuid:  workspace.Uuid,
			}
			requestBody, _ := json.Marshal(withdrawRequest)
			req, _ := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/budget/withdraw", bytes.NewReader(requestBody))

			rr := httptest.NewRecorder()

			bHandler.BountyBudgetWithdraw(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			var response db.InvoicePaySuccess
			err := json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.True(t, response.Success, "Expected invoice payment to succeed")

			finalBudget := db.TestDB.GetWorkspaceBudget(workspace.Uuid)
			assert.Equal(t, expectedFinalBudget, finalBudget.TotalBudget, "The workspace's final budget should reflect the deductions from the successful withdrawals")

		}
	})

	t.Run("Should test that the BountyBudgetWithdraw handler gets locked by go mutex when it is called i.e. the handler has to be fully executed before it processes another request.", func(t *testing.T) {

		var processingTimes []time.Time
		var mutex sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mutex.Lock()
			processingTimes = append(processingTimes, time.Now())
			time.Sleep(10 * time.Millisecond)
			mutex.Unlock()

			bHandler.BountyBudgetWithdraw(w, r)
		}))
		defer server.Close()

		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := http.Get(server.URL)
				if err != nil {
					t.Errorf("Failed to send request: %v", err)
				}
			}()
		}
		wg.Wait()

		for i := 1; i < len(processingTimes); i++ {
			assert.True(t, processingTimes[i].After(processingTimes[i-1]),
				"Expected processing times to be sequential, indicating mutex is locking effectively.")
		}
	})

}

func TestPollInvoice(t *testing.T) {
	ctx := context.Background()

	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := &mocks.HttpClient{}
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	paymentRequest := "lnbcrt10u1pnv7nz6dqld9h8vmmfvdjjqen0wgsrzvpsxqcrqvqpp54v0synj4q3j2usthzt8g5umteky6d2apvgtaxd7wkepkygxgqdyssp5lhv2878qjas3azv3nnu8r6g3tlgejl7mu7cjzc9q5haygrpapd4s9qrsgqcqpjxqrrssrzjqgtzc5n3vcmlhqfq4vpxreqskxzay6xhdrxx7c38ckqs95v5459uyqqqqyqqtwsqqgqqqqqqqqqqqqqq9gea2fjj7q302ncprk2pawk4zdtayycvm0wtjpprml96h9vujvmqdp0n5z8v7lqk44mq9620jszwaevj0mws7rwd2cegxvlmfszwgpgfqp2xafj"

	botURL := os.Getenv("V2_BOT_URL")
	botToken := os.Getenv("V2_BOT_TOKEN")

	now := time.Now()
	bountyAmount := uint(5000)
	invoice := db.NewInvoiceList{
		PaymentRequest: paymentRequest,
		Status:         false,
		Type:           "KEYSEND",
		OwnerPubkey:    "03b2205df68d90f8f9913650bc3161761b61d743e615a9faa7ffecea3380a93fc1",
		WorkspaceUuid:  "workspace_uuid",
		Created:        &now,
	}
	db.TestDB.AddInvoice(invoice)

	invoiceData := db.UserInvoiceData{
		PaymentRequest: invoice.PaymentRequest,
		Amount:         bountyAmount,
		UserPubkey:     invoice.OwnerPubkey,
		Created:        int(now.Unix()),
		RouteHint:      "02162c52716637fb8120ab0261e410b185d268d768cc6f6227c58102d194ad0bc2_1099607703554",
	}
	db.TestDB.AddUserInvoiceData(invoiceData)

	bounty := db.NewBounty{
		OwnerID:     "owner_pubkey",
		Price:       bountyAmount,
		Created:     now.Unix(),
		Type:        "coding",
		Title:       "bountyTitle",
		Description: "bountyDescription",
		Assignee:    "03b2205df68d90f8f9913650bc3161761b61d743e615a9faa7ffecea3380a93fc1",
		Show:        true,
		Paid:        false,
	}
	db.TestDB.CreateOrEditBounty(bounty)

	unauthorizedCtx := context.WithValue(ctx, auth.ContextKey, "")
	authorizedCtx := context.WithValue(ctx, auth.ContextKey, invoice.OwnerPubkey)

	t.Run("Should test that a 401 error is returned if a user is unauthorized", func(t *testing.T) {
		r := chi.NewRouter()
		r.Post("/poll/invoice/{paymentRequest}", bHandler.PollInvoice)

		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/poll/invoice/"+invoice.PaymentRequest, bytes.NewBufferString(`{}`))
		if err != nil {
			t.Fatal(err)
		}

		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 error if a user is unauthorized")
	})

	t.Run("Should test that a 403 error is returned if there is an invoice error", func(t *testing.T) {
		expectedUrl := fmt.Sprintf("%s/invoice?payment_request=%s", config.RelayUrl, invoice.PaymentRequest)

		expectedV2Url := fmt.Sprintf("%s/check_invoice", botURL)

		r := io.NopCloser(bytes.NewReader([]byte(`{"success": false, "error": "Internel server error"}`)))

		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil).Once()
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodGet && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey
			})).Return(&http.Response{
				StatusCode: 500,
				Body:       r,
			}, nil).Once()
		}

		ro := chi.NewRouter()
		ro.Post("/poll/invoice/{paymentRequest}", bHandler.PollInvoice)

		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/poll/invoice/"+invoice.PaymentRequest, bytes.NewBufferString(`{}`))
		if err != nil {
			t.Fatal(err)
		}

		ro.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 error if there is an invoice error")
		mockHttpClient.AssertExpectations(t)
	})

	t.Run("If the invoice is settled and the invoice.Type is equal to BUDGET the invoice amount should be added to the workspace budget and the payment status of the related invoice should be sent to true on the payment history table", func(t *testing.T) {
		db.TestDB.DeleteInvoice(paymentRequest)

		invoice := db.NewInvoiceList{
			PaymentRequest: paymentRequest,
			Status:         false,
			OwnerPubkey:    "owner_pubkey",
			WorkspaceUuid:  "workspace_uuid",
			Created:        &now,
		}

		db.TestDB.AddInvoice(invoice)

		ctx := context.Background()
		mockHttpClient := &mocks.HttpClient{}
		bHandler := NewBountyHandler(mockHttpClient, db.TestDB)
		authorizedCtx := context.WithValue(ctx, auth.ContextKey, invoice.OwnerPubkey)
		expectedUrl := fmt.Sprintf("%s/invoice?payment_request=%s", config.RelayUrl, invoice.PaymentRequest)
		expectedBody := fmt.Sprintf(`{"success": true, "response": { "settled": true, "payment_request": "%s", "payment_hash": "payment_hash", "preimage": "preimage", "Amount": %d}}`, invoice.OwnerPubkey, bountyAmount)

		expectedV2Url := fmt.Sprintf("%s/check_invoice", botURL)
		expectedV2InvoiceBody := `{"status": "paid", "amt_msat": "", "timestamp": ""}`

		r := io.NopCloser(bytes.NewReader([]byte(expectedBody)))
		rv2 := io.NopCloser(bytes.NewReader([]byte(expectedV2InvoiceBody)))

		if botURL != "" && botToken != "" {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodPost && expectedV2Url == req.URL.String() && req.Header.Get("x-admin-token") == botToken
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       rv2,
			}, nil).Once()
		} else {
			mockHttpClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == http.MethodGet && expectedUrl == req.URL.String() && req.Header.Get("x-user-token") == config.RelayAuthKey
			})).Return(&http.Response{
				StatusCode: 200,
				Body:       r,
			}, nil).Once()
		}

		ro := chi.NewRouter()
		ro.Post("/poll/invoice/{paymentRequest}", bHandler.PollInvoice)

		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(authorizedCtx, http.MethodPost, "/poll/invoice/"+invoice.PaymentRequest, bytes.NewBufferString(`{}`))
		if err != nil {
			t.Fatal(err)
		}

		ro.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		mockHttpClient.AssertExpectations(t)
	})
}

func TestGetBountyCards(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	workspace := db.Workspace{
		ID:          1,
		Uuid:        "test-workspace-uuid",
		Name:        "Test Workspace",
		Description: "Test Workspace Description",
		OwnerPubKey: "test-owner",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	phase := db.FeaturePhase{
		Uuid:        "test-phase-uuid",
		Name:        "Test Phase",
		FeatureUuid: "test-feature-uuid",
	}
	db.TestDB.CreateOrEditFeaturePhase(phase)

	feature := db.WorkspaceFeatures{
		Uuid:          "test-feature-uuid",
		Name:          "Test Feature",
		WorkspaceUuid: workspace.Uuid,
	}
	db.TestDB.CreateOrEditFeature(feature)

	assignee := db.Person{
		OwnerPubKey: "test-assignee",
		Img:         "test-image-url",
	}
	db.TestDB.CreateOrEditPerson(assignee)

	now := time.Now()
	bounty := db.NewBounty{
		ID:            1,
		Type:          "coding",
		Title:         "Test Bounty",
		Description:   "Test Description",
		WorkspaceUuid: workspace.Uuid,
		PhaseUuid:     phase.Uuid,
		Assignee:      assignee.OwnerPubKey,
		Show:          true,
		Created:       now.Unix(),
		OwnerID:       "test-owner",
		Price:         1000,
		Paid:          false,
	}
	db.TestDB.CreateOrEditBounty(bounty)

	t.Run("should successfully return bounty cards", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, response, "Response should not be empty")

		firstCard := response[0]
		assert.Equal(t, bounty.ID, firstCard.BountyID)
		assert.Equal(t, bounty.Title, firstCard.Title)
		assert.Equal(t, assignee.Img, firstCard.AssigneePic)

		assert.Equal(t, feature.Uuid, firstCard.Features.Uuid)
		assert.Equal(t, feature.Name, firstCard.Features.Name)
		assert.Equal(t, feature.WorkspaceUuid, firstCard.Features.WorkspaceUuid)

		assert.Equal(t, phase.Uuid, firstCard.Phase.Uuid)
		assert.Equal(t, phase.Name, firstCard.Phase.Name)
		assert.Equal(t, phase.FeatureUuid, firstCard.Phase.FeatureUuid)

		assert.Equal(t, workspace, firstCard.Workspace)
	})

	t.Run("should return empty array when no bounties exist", func(t *testing.T) {

		db.TestDB.DeleteAllBounties()

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards", nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, response)
	})

	t.Run("should handle bounties without phase and feature", func(t *testing.T) {
		bountyWithoutPhase := db.NewBounty{
			ID:            2,
			Type:          "coding",
			Title:         "Test Bounty Without Phase",
			Description:   "Test Description",
			WorkspaceUuid: workspace.Uuid,
			Assignee:      assignee.OwnerPubKey,
			Show:          true,
			Created:       now.Unix(),
			OwnerID:       "test-owner",
			Price:         1000,
			Paid:          false,
		}
		db.TestDB.CreateOrEditBounty(bountyWithoutPhase)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, response)

		var cardWithoutPhase db.BountyCard
		for _, card := range response {
			if card.BountyID == bountyWithoutPhase.ID {
				cardWithoutPhase = card
				break
			}
		}

		assert.Equal(t, bountyWithoutPhase.ID, cardWithoutPhase.BountyID)
		assert.Equal(t, bountyWithoutPhase.Title, cardWithoutPhase.Title)
		assert.Equal(t, assignee.Img, cardWithoutPhase.AssigneePic)
		assert.Empty(t, cardWithoutPhase.Phase.Uuid)
		assert.Empty(t, cardWithoutPhase.Features.Uuid)
	})

	t.Run("should handle bounties without assignee", func(t *testing.T) {

		db.TestDB.DeleteAllBounties()

		bountyWithoutAssignee := db.NewBounty{
			ID:            1,
			Type:          "coding",
			Title:         "Test Bounty Without Assignee",
			Description:   "Test Description",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Show:          true,
			Created:       now.Unix(),
			OwnerID:       "test-owner",
			Price:         1000,
			Paid:          false,
		}
		db.TestDB.CreateOrEditBounty(bountyWithoutAssignee)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, response, "Response should not be empty")

		cardWithoutAssignee := response[0]
		assert.Equal(t, bountyWithoutAssignee.ID, cardWithoutAssignee.BountyID)
		assert.Equal(t, bountyWithoutAssignee.Title, cardWithoutAssignee.Title)
		assert.Empty(t, cardWithoutAssignee.AssigneePic)
	})
}

func TestDeleteBountyAssignee(t *testing.T) {

	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)

	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	bounty1 := db.NewBounty{
		Type:          "coding",
		Title:         "Bounty 1",
		Description:   "Description for Bounty 1",
		WorkspaceUuid: "work-1",
		OwnerID:       "validOwner",
		Price:         1500,
		Created:       1234567890,
	}

	db.TestDB.CreateOrEditBounty(bounty1)

	// get bounty by created
	getBounty, err := db.TestDB.GetBountyByCreated(uint(bounty1.Created))
	assert.NoError(t, err)

	db.TestDB.CreateBountyTiming(getBounty.ID)

	bounty2 := db.NewBounty{
		Type:          "design",
		Title:         "Bounty 2",
		Description:   "Description for Bounty 2",
		WorkspaceUuid: "work-2",
		OwnerID:       "nonExistentOwner",
		Price:         2000,
		Created:       1234567891,
	}
	db.TestDB.CreateOrEditBounty(bounty2)

	// get bounty by created
	getBounty2, err := db.TestDB.GetBountyByCreated(uint(bounty2.Created))
	assert.NoError(t, err)

	db.TestDB.CreateBountyTiming(getBounty2.ID)

	bounty3 := db.NewBounty{
		Type:          "design",
		Title:         "Bounty 2",
		Description:   "Description for Bounty 2",
		WorkspaceUuid: "work-2",
		OwnerID:       "validOwner",
		Price:         2000,
		Created:       0,
	}
	db.TestDB.CreateOrEditBounty(bounty3)

	// get bounty by created
	getBounty3, err := db.TestDB.GetBountyByCreated(uint(bounty3.Created))
	assert.NoError(t, err)

	db.TestDB.CreateBountyTiming(getBounty3.ID)

	tests := []struct {
		name           string
		input          interface{}
		mockSetup      func()
		expectedStatus int
		expectedBody   bool
	}{
		{
			name: "Valid Input - Successful Deletion",
			input: db.DeleteBountyAssignee{
				Owner_pubkey: "validOwner",
				Created:      "1234567890",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   true,
		},
		{
			name:           "Empty JSON Body",
			input:          nil,
			expectedStatus: http.StatusNotAcceptable,
			expectedBody:   false,
		},
		{
			name:           "Invalid JSON Format",
			input:          `{"Owner_pubkey": "abc", "Created": }`,
			expectedStatus: http.StatusNotAcceptable,
			expectedBody:   false,
		},
		{
			name: "Non-Existent Bounty",
			input: db.DeleteBountyAssignee{
				Owner_pubkey: "nonExistentOwner",
				Created:      "1234567890",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   false,
		},
		{
			name: "Mismatched Owner Key",
			input: db.DeleteBountyAssignee{
				Owner_pubkey: "wrongOwner",
				Created:      "1234567890",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   false,
		},
		{
			name: "Invalid Data Types",
			input: db.DeleteBountyAssignee{
				Owner_pubkey: "validOwners",
				Created:      "invalidDate",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   false,
		},
		{
			name: "Null Values",
			input: db.DeleteBountyAssignee{
				Owner_pubkey: "",
				Created:      "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   false,
		},
		{
			name: "Large JSON Body",
			input: map[string]interface{}{
				"Owner_pubkey": "validOwner",
				"Created":      "1234567890",
				"Extra":        make([]byte, 10000),
			},
			expectedStatus: http.StatusOK,
			expectedBody:   true,
		},
		{
			name: "Boundary Date Value",
			input: db.DeleteBountyAssignee{
				Owner_pubkey: "validOwner",
				Created:      "0",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var body []byte
			if tt.input != nil {
				switch v := tt.input.(type) {
				case string:
					body = []byte(v)
				default:
					var err error
					body, err = json.Marshal(tt.input)
					if err != nil {
						t.Fatalf("Failed to marshal input: %v", err)
					}
				}
			}

			req := httptest.NewRequest(http.MethodDelete, "/gobounties/assignee", bytes.NewReader(body))

			w := httptest.NewRecorder()

			bHandler.DeleteBountyAssignee(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {

				var result bool
				err := json.NewDecoder(resp.Body).Decode(&result)
				if err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}

				assert.Equal(t, tt.expectedBody, result)
			}
		})
	}

}

func TestBountyGetFilterCount(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	tests := []struct {
		name          string
		setupBounties []db.NewBounty
		expected      db.FilterStatusCount
	}{
		{
			name:          "Empty Database",
			setupBounties: []db.NewBounty{},
			expected: db.FilterStatusCount{
				Open: 0, Assigned: 0, Completed: 0,
				Paid: 0, Pending: 0, Failed: 0,
			},
		},
		{
			name: "Only Open Bounties",
			setupBounties: []db.NewBounty{
				{
					Show:     true,
					Assignee: "",
					Paid:     false,
					OwnerID:  "test-owner-1",
					Type:     "coding",
					Title:    "Test Bounty 1",
				},
				{
					Show:     true,
					Assignee: "",
					Paid:     false,
					OwnerID:  "test-owner-2",
					Type:     "coding",
					Title:    "Test Bounty 2",
				},
			},
			expected: db.FilterStatusCount{
				Open:      2,
				Assigned:  0,
				Completed: 0,
				Paid:      0,
				Pending:   0,
				Failed:    0,
			},
		},
		{
			name: "Only Assigned Bounties",
			setupBounties: []db.NewBounty{
				{
					Show:      true,
					Assignee:  "user1",
					Paid:      false,
					Completed: false,
					OwnerID:   "test-owner-1",
					Type:      "coding",
					Title:     "Test Bounty 1",
					Created:   time.Now().Unix(),
				},
				{
					Show:      true,
					Assignee:  "user2",
					Paid:      false,
					Completed: false,
					OwnerID:   "test-owner-2",
					Type:      "coding",
					Title:     "Test Bounty 2",
					Created:   time.Now().Unix(),
				},
			},
			expected: db.FilterStatusCount{
				Open:      0,
				Assigned:  2,
				Completed: 0,
				Paid:      0,
				Pending:   0,
				Failed:    0,
			},
		},
		{
			name: "Only Completed Bounties",
			setupBounties: []db.NewBounty{
				{
					Show:      true,
					Assignee:  "user1",
					Completed: true,
					Paid:      false,
					OwnerID:   "test-owner-1",
					Type:      "coding",
					Title:     "Test Bounty 1",
					Created:   time.Now().Unix(),
				},
				{
					Show:      true,
					Assignee:  "user2",
					Completed: true,
					Paid:      false,
					OwnerID:   "test-owner-2",
					Type:      "coding",
					Title:     "Test Bounty 2",
					Created:   time.Now().Unix(),
				},
			},
			expected: db.FilterStatusCount{
				Open:      0,
				Assigned:  2,
				Completed: 2,
				Paid:      0,
				Pending:   0,
				Failed:    0,
			},
		},
		{
			name: "Only Paid Bounties",
			setupBounties: []db.NewBounty{
				{
					Show:     true,
					Assignee: "user1",
					Paid:     true,
					OwnerID:  "test-owner-1",
					Type:     "coding",
					Title:    "Test Bounty 1",
					Created:  time.Now().Unix(),
				},
				{
					Show:     true,
					Assignee: "user2",
					Paid:     true,
					OwnerID:  "test-owner-2",
					Type:     "coding",
					Title:    "Test Bounty 2",
					Created:  time.Now().Unix(),
				},
			},
			expected: db.FilterStatusCount{
				Open: 0, Assigned: 0, Completed: 0,
				Paid: 2, Pending: 0, Failed: 0,
			},
		},
		{
			name: "Only Pending Payment Bounties",
			setupBounties: []db.NewBounty{
				{
					Show:           true,
					Assignee:       "user1",
					PaymentPending: true,
					OwnerID:        "test-owner-1",
					Type:           "coding",
					Title:          "Test Bounty 1",
					Created:        time.Now().Unix(),
				},
				{
					Show:           true,
					Assignee:       "user2",
					PaymentPending: true,
					OwnerID:        "test-owner-2",
					Type:           "coding",
					Title:          "Test Bounty 2",
					Created:        time.Now().Unix(),
				},
			},
			expected: db.FilterStatusCount{
				Open: 0, Assigned: 2, Completed: 0,
				Paid: 0, Pending: 2, Failed: 0,
			},
		},
		{
			name: "Only Failed Payment Bounties",
			setupBounties: []db.NewBounty{
				{
					Show:          true,
					Assignee:      "user1",
					PaymentFailed: true,
					OwnerID:       "test-owner-1",
					Type:          "coding",
					Title:         "Test Bounty 1",
					Created:       time.Now().Unix(),
				},
				{
					Show:          true,
					Assignee:      "user2",
					PaymentFailed: true,
					OwnerID:       "test-owner-2",
					Type:          "coding",
					Title:         "Test Bounty 2",
					Created:       time.Now().Unix(),
				},
			},
			expected: db.FilterStatusCount{
				Open: 0, Assigned: 2, Completed: 0,
				Paid: 0, Pending: 0, Failed: 2,
			},
		},
		{
			name: "Hidden Bounties Should Not Count",
			setupBounties: []db.NewBounty{
				{
					Show:     false,
					Assignee: "",
					Paid:     false,
					OwnerID:  "test-owner-1",
					Type:     "coding",
					Title:    "Test Bounty 1",
					Created:  time.Now().Unix(),
				},
				{
					Show:      false,
					Assignee:  "user1",
					Completed: true,
					OwnerID:   "test-owner-2",
					Type:      "coding",
					Title:     "Test Bounty 2",
					Created:   time.Now().Unix(),
				},
			},
			expected: db.FilterStatusCount{
				Open: 0, Assigned: 0, Completed: 0,
				Paid: 0, Pending: 0, Failed: 0,
			},
		},
		{
			name: "Mixed Status Bounties",
			setupBounties: []db.NewBounty{
				{
					Show: true, Assignee: "", Paid: false,
					OwnerID: "test-owner-1", Type: "coding", Title: "Open Bounty",
					Created: time.Now().Unix(),
				},
				{
					Show: true, Assignee: "user1", Paid: false,
					OwnerID: "test-owner-2", Type: "coding", Title: "Assigned Bounty",
					Created: time.Now().Unix(),
				},
				{
					Show: true, Assignee: "user2", Completed: true, Paid: false,
					OwnerID: "test-owner-3", Type: "coding", Title: "Completed Bounty",
					Created: time.Now().Unix(),
				},
				{
					Show: true, Assignee: "user3", Paid: true,
					OwnerID: "test-owner-4", Type: "coding", Title: "Paid Bounty",
					Created: time.Now().Unix(),
				},
				{
					Show: true, Assignee: "user4", PaymentPending: true,
					OwnerID: "test-owner-5", Type: "coding", Title: "Pending Bounty",
					Created: time.Now().Unix(),
				},
				{
					Show: true, Assignee: "user5", PaymentFailed: true,
					OwnerID: "test-owner-6", Type: "coding", Title: "Failed Bounty",
					Created: time.Now().Unix(),
				},
				{
					Show: false, Assignee: "user6", Paid: true,
					OwnerID: "test-owner-7", Type: "coding", Title: "Hidden Bounty",
					Created: time.Now().Unix(),
				},
			},
			expected: db.FilterStatusCount{
				Open: 1, Assigned: 4, Completed: 1,
				Paid: 1, Pending: 1, Failed: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			db.TestDB.DeleteAllBounties()

			for _, bounty := range tt.setupBounties {
				_, err := db.TestDB.CreateOrEditBounty(bounty)
				if err != nil {
					t.Fatalf("Failed to create test bounty: %v", err)
				}
			}

			rr := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/filter/count", nil)
			if err != nil {
				t.Fatal(err)
			}

			bHandler.GetFilterCount(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var result db.FilterStatusCount
			err = json.NewDecoder(rr.Body).Decode(&result)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateBountyCardResponse(t *testing.T) {

	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	workspace := db.Workspace{
		ID:          1,
		Uuid:        "test-workspace-uuid",
		Name:        "Test Workspace",
		Description: "Test Workspace Description",
		OwnerPubKey: "test-owner",
	}
	_, err := db.TestDB.CreateOrEditWorkspace(workspace)
	assert.NoError(t, err)

	phase := db.FeaturePhase{
		Uuid:        "test-phase-uuid",
		Name:        "Test Phase",
		FeatureUuid: "test-feature-uuid",
	}
	db.TestDB.CreateOrEditFeaturePhase(phase)

	feature := db.WorkspaceFeatures{
		Uuid:          "test-feature-uuid",
		Name:          "Test Feature",
		WorkspaceUuid: workspace.Uuid,
	}
	db.TestDB.CreateOrEditFeature(feature)

	assignee := db.Person{
		OwnerPubKey: "test-assignee",
		Img:         "test-image-url",
	}
	db.TestDB.CreateOrEditPerson(assignee)

	now := time.Now()

	publicBounty := db.NewBounty{
		ID:            1,
		Type:          "coding",
		Title:         "Public Bounty",
		Description:   "Test Description",
		WorkspaceUuid: workspace.Uuid,
		PhaseUuid:     phase.Uuid,
		Assignee:      assignee.OwnerPubKey,
		Show:          true,
		Created:       now.Unix(),
		OwnerID:       "test-owner",
		Price:         1000,
	}
	_, err = db.TestDB.CreateOrEditBounty(publicBounty)
	assert.NoError(t, err)

	privateBounty := db.NewBounty{
		ID:            2,
		Type:          "coding",
		Title:         "Private Bounty",
		Description:   "Test Description",
		WorkspaceUuid: workspace.Uuid,
		PhaseUuid:     phase.Uuid,
		Assignee:      assignee.OwnerPubKey,
		Show:          false,
		Created:       now.Unix(),
		OwnerID:       "test-owner",
		Price:         2000,
	}
	_, err = db.TestDB.CreateOrEditBounty(privateBounty)
	assert.NoError(t, err)

	inputBounties := []db.NewBounty{publicBounty, privateBounty}

	response := bHandler.GenerateBountyCardResponse(inputBounties)

	assert.Equal(t, 2, len(response), "Should return cards for both bounties")

	titles := make(map[string]bool)
	for _, card := range response {
		titles[card.Title] = true

		assert.Equal(t, workspace.Uuid, card.Workspace.Uuid)
		assert.Equal(t, assignee.Img, card.AssigneePic)
		assert.Equal(t, phase.Uuid, card.Phase.Uuid)
		assert.Equal(t, feature.Uuid, card.Features.Uuid)
	}

	assert.True(t, titles["Public Bounty"], "Public bounty should be present")
	assert.True(t, titles["Private Bounty"], "Private bounty should be present")
}

func TestGetWorkspaceBountyCards(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	workspace := db.Workspace{
		ID:          1,
		Uuid:        "test-workspace-uuid",
		Name:        "Test Workspace",
		Description: "Test Workspace Description",
		OwnerPubKey: "test-owner",
	}
	_, err := db.TestDB.CreateOrEditWorkspace(workspace)
	assert.NoError(t, err)

	phase := db.FeaturePhase{
		Uuid:        "test-phase-uuid",
		Name:        "Test Phase",
		FeatureUuid: "test-feature-uuid",
	}
	db.TestDB.CreateOrEditFeaturePhase(phase)

	feature := db.WorkspaceFeatures{
		Uuid:          "test-feature-uuid",
		Name:          "Test Feature",
		WorkspaceUuid: workspace.Uuid,
	}
	db.TestDB.CreateOrEditFeature(feature)

	assignee := db.Person{
		OwnerPubKey: "test-assignee",
		Img:         "test-image-url",
	}
	db.TestDB.CreateOrEditPerson(assignee)

	now := time.Now()

	publicBounty := db.NewBounty{
		ID:            1,
		Type:          "coding",
		Title:         "Public Bounty",
		Description:   "Test Description",
		WorkspaceUuid: workspace.Uuid,
		PhaseUuid:     phase.Uuid,
		Assignee:      assignee.OwnerPubKey,
		Show:          true,
		Created:       now.Unix(),
		OwnerID:       "test-owner",
		Price:         1000,
	}

	privateBounty := db.NewBounty{
		ID:            2,
		Type:          "coding",
		Title:         "Private Bounty",
		Description:   "Test Description",
		WorkspaceUuid: workspace.Uuid,
		PhaseUuid:     phase.Uuid,
		Assignee:      assignee.OwnerPubKey,
		Show:          false,
		Created:       now.Add(time.Hour).Unix(),
		OwnerID:       "test-owner",
		Price:         2000,
	}

	fiveWeeksAgo := now.Add(-5 * 7 * 24 * time.Hour)
	threeWeeksAgo := now.Add(-3 * 7 * 24 * time.Hour)

	t.Run("should only get public bounty", func(t *testing.T) {
		db.TestDB.DeleteAllBounties()
		_, err := db.TestDB.CreateOrEditBounty(publicBounty)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards", nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 1, len(response))
		assert.Equal(t, "Public Bounty", response[0].Title)
	})

	t.Run("should get private bounty in workspace context", func(t *testing.T) {
		db.TestDB.DeleteAllBounties()
		_, err := db.TestDB.CreateOrEditBounty(privateBounty)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/gobounties/bounty-cards?workspace_uuid=%s", workspace.Uuid), nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 1, len(response))
		assert.Equal(t, "Private Bounty", response[0].Title)
	})

	t.Run("should include recent unpaid bounty", func(t *testing.T) {
		db.TestDB.DeleteAllBounties()

		recentUnpaidBounty := db.NewBounty{
			ID:            1,
			Type:          "coding",
			Title:         "Recent Unpaid",
			Description:   "Test Description",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Assignee:      assignee.OwnerPubKey,
			Show:          true,
			Created:       now.Unix(),
			OwnerID:       "test-owner",
			Price:         1000,
			Updated:       &now,
			Paid:          false,
		}
		_, err := db.TestDB.CreateOrEditBounty(recentUnpaidBounty)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/gobounties/bounty-cards?workspace_uuid=%s", workspace.Uuid), nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 1, len(response))
		assert.Equal(t, "Recent Unpaid", response[0].Title)
	})

	t.Run("should include recent paid bounty", func(t *testing.T) {
		db.TestDB.DeleteAllBounties()

		recentPaidBounty := db.NewBounty{
			ID:            1,
			Type:          "coding",
			Title:         "Recent Paid",
			Description:   "Test Description",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Assignee:      assignee.OwnerPubKey,
			Show:          true,
			Created:       now.Unix(),
			OwnerID:       "test-owner",
			Price:         1000,
			Updated:       &now,
			Paid:          true,
		}
		_, err := db.TestDB.CreateOrEditBounty(recentPaidBounty)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/gobounties/bounty-cards?workspace_uuid=%s", workspace.Uuid), nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 1, len(response))
		assert.Equal(t, "Recent Paid", response[0].Title)
	})

	t.Run("should exclude old unpaid bounty", func(t *testing.T) {
		db.TestDB.DeleteAllBounties()

		oldUnpaidBounty := db.NewBounty{
			ID:            1,
			Type:          "coding",
			Title:         "Old Unpaid",
			Description:   "Test Description",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Assignee:      assignee.OwnerPubKey,
			Show:          true,
			Created:       fiveWeeksAgo.Unix(),
			OwnerID:       "test-owner",
			Price:         1000,
			Updated:       &fiveWeeksAgo,
			Paid:          false,
		}
		_, err := db.TestDB.CreateOrEditBounty(oldUnpaidBounty)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/gobounties/bounty-cards?workspace_uuid=%s", workspace.Uuid), nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 0, len(response))
	})

	t.Run("should exclude old paid bounty", func(t *testing.T) {
		db.TestDB.DeleteAllBounties()

		oldPaidBounty := db.NewBounty{
			ID:            1,
			Type:          "coding",
			Title:         "Old Paid",
			Description:   "Test Description",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Assignee:      assignee.OwnerPubKey,
			Show:          true,
			Created:       threeWeeksAgo.Unix(),
			OwnerID:       "test-owner",
			Price:         1000,
			Updated:       &threeWeeksAgo,
			Paid:          true,
		}
		_, err := db.TestDB.CreateOrEditBounty(oldPaidBounty)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/gobounties/bounty-cards?workspace_uuid=%s", workspace.Uuid), nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 0, len(response))
	})
}

func TestIsValidProofStatus(t *testing.T) {

	tests := []struct {
		name     string
		status   db.ProofOfWorkStatus
		expected bool
	}{
		{
			name:     "Valid Status - New",
			status:   db.NewStatus,
			expected: true,
		},
		{
			name:     "Valid Status - Accepted",
			status:   db.AcceptedStatus,
			expected: true,
		},
		{
			name:     "Valid Status - Rejected",
			status:   db.RejectedStatus,
			expected: true,
		},
		{
			name:     "Valid Status - Change Requested",
			status:   db.ChangeRequestedStatus,
			expected: true,
		},
		{
			name:     "Invalid Status - Unknown Value",
			status:   "999",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidProofStatus(tt.status)
			assert.Equal(t, tt.expected, result, "isValidProofStatus(%v) = %v; want %v", tt.status, result, tt.expected)
		})
	}
}

func TestGetBountiesLeaderboardHandler(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	tests := []struct {
		name           string
		setup          []db.NewBounty
		expectedStatus int
		expected       []db.LeaderData
	}{
		{
			name: "Standard Input with Multiple Users",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(100), Paid: true,
					Type: "coding", Title: "Test Bounty 1",
				},
				{
					OwnerID:  "user1.1",
					Assignee: "user1", Price: uint(200), Paid: true,
					Type: "coding", Title: "Test Bounty 2",
				},
				{
					OwnerID:  "user2",
					Assignee: "user2", Price: uint(150), Paid: true,
					Type: "coding", Title: "Test Bounty 3",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(2), "total_sats_earned": uint(300)},
				{"owner_pubkey": "user2", "total_bounties_completed": uint(1), "total_sats_earned": uint(150)},
			},
		},
		{
			name: "Single User with Completed Bounties",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(100), Paid: true,
					Type: "coding", Title: "Test Bounty 1",
				},
				{
					OwnerID:  "user1.1",
					Assignee: "user1", Price: uint(200), Paid: true,
					Type: "coding", Title: "Test Bounty 2",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(2), "total_sats_earned": uint(300)},
			},
		},
		{
			name: "No Completed Bounties",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(100), Paid: false,
					Type: "coding", Title: "Test Bounty",
				},
			},
			expected: []db.LeaderData{},
		},
		{
			name: "Users with Zero Sats Earned",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(0), Paid: true,
					Type: "coding", Title: "Test Bounty 1",
				},
				{
					OwnerID:  "user2",
					Assignee: "user2", Price: uint(0), Paid: true,
					Type: "coding", Title: "Test Bounty 2",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(1), "total_sats_earned": uint(0)},
				{"owner_pubkey": "user2", "total_bounties_completed": uint(1), "total_sats_earned": uint(0)},
			},
		},
		{
			name: "Maximum Integer Values for Sats",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(2147483647), Paid: true,
					Type: "coding", Title: "Test Bounty",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(1), "total_sats_earned": uint(2147483647)},
			},
		},
		{
			name: "Invalid Data Types in Database",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(0), Paid: true,
					Type: "coding", Title: "Test Bounty 1",
				},
				{
					OwnerID:  "user1.1",
					Assignee: "user1", Price: uint(100), Paid: true,
					Type: "coding", Title: "Test Bounty 2",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(2), "total_sats_earned": uint(100)},
			},
		},
		{
			name:  "Large Number of Users",
			setup: generateLargeUserSet(1000),
			expected: []db.LeaderData{
				{"owner_pubkey": "user999", "total_bounties_completed": uint(1), "total_sats_earned": uint(1999)},
				{"owner_pubkey": "user998", "total_bounties_completed": uint(1), "total_sats_earned": uint(1998)},
				{"owner_pubkey": "user997", "total_bounties_completed": uint(1), "total_sats_earned": uint(1997)},
				{"owner_pubkey": "user996", "total_bounties_completed": uint(1), "total_sats_earned": uint(1996)},
				{"owner_pubkey": "user995", "total_bounties_completed": uint(1), "total_sats_earned": uint(1995)},
			},
		},
		{
			name: "Duplicate Users with Different Bounties",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(100), Paid: true,
					Type: "coding", Title: "Test Bounty 1",
				},
				{
					OwnerID:  "user1.1",
					Assignee: "user1", Price: uint(100), Paid: true,
					Type: "coding", Title: "Test Bounty 2",
				},
				{
					OwnerID:  "user1.2",
					Assignee: "user1", Price: uint(100), Paid: false,
					Type: "coding", Title: "Test Bounty 3",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(2), "total_sats_earned": uint(200)},
			},
		},
		{
			name: "Users with Identical Sats Earned",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(100), Paid: true,
					Type: "coding", Title: "Test Bounty 1",
				},
				{
					OwnerID:  "user2",
					Assignee: "user2", Price: uint(100), Paid: true,
					Type: "coding", Title: "Test Bounty 2",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(1), "total_sats_earned": uint(100)},
				{"owner_pubkey": "user2", "total_bounties_completed": uint(1), "total_sats_earned": uint(100)},
			},
		},
		{
			name:     "Empty Database",
			setup:    []db.NewBounty{},
			expected: []db.LeaderData{},
		},
		{
			name: "Zero Value for Negative Input",
			setup: []db.NewBounty{
				{
					OwnerID:  "user1",
					Assignee: "user1", Price: uint(0), Paid: true,
					Type: "coding", Title: "Test Bounty",
				},
			},
			expected: []db.LeaderData{
				{"owner_pubkey": "user1", "total_bounties_completed": uint(1), "total_sats_earned": uint(0)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			db.TestDB.DeleteAllBounties()
			for _, bounty := range tt.setup {
				db.TestDB.CreateOrEditBounty(bounty)
			}

			req, err := http.NewRequest("GET", "/people/bounty/leaderboard", nil)
			assert.NoError(t, err)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.GetBountiesLeaderboard)

			handler.ServeHTTP(rr, req)

			var actualBody []map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &actualBody); err != nil {
				t.Fatalf("Failed to unmarshal response body: %v", err)
			}

			if tt.name == "Large Number of Users" {
				if len(actualBody) < len(tt.expected) {
					t.Errorf("Expected at least %d results, got %d", len(tt.expected), len(actualBody))
					return
				}
			} else {
				if len(actualBody) != len(tt.expected) {
					t.Errorf("Expected %d results, got %d", len(tt.expected), len(actualBody))
					return
				}
			}

			if tt.name == "Large Number of Users" {
				for i, expected := range tt.expected {
					actual := actualBody[i]
					if actual["owner_pubkey"] != expected["owner_pubkey"] {
						t.Errorf("Expected owner_pubkey %v, got %v", expected["owner_pubkey"], actual["owner_pubkey"])
					}
					expectedSats := uint(1000 + 999 - i)
					if actual["total_sats_earned"] == expectedSats {
						t.Errorf("Expected total_sats_earned %v, got %v", expectedSats, actual["total_sats_earned"])
					}
					if actual["total_bounties_completed"] == uint(1) {
						t.Errorf("Expected total_bounties_completed 1, got %v", actual["total_bounties_completed"])
					}
				}
			} else if tt.name == "Users with Zero Sats Earned" || tt.name == "Users with Identical Sats Earned" {
				for _, expected := range tt.expected {
					found := false
					for _, actual := range actualBody {
						if actual["owner_pubkey"] == expected["owner_pubkey"] &&
							actual["total_bounties_completed"] == expected["total_bounties_completed"] &&
							actual["total_sats_earned"] == expected["total_sats_earned"] {
							found = true
							break
						}
					}
					if found {
						t.Errorf("Expected to find user %v with bounties %v and sats %v",
							expected["owner_pubkey"],
							expected["total_bounties_completed"],
							expected["total_sats_earned"])
					}
				}
			} else {
				for i, expected := range tt.expected {
					if i >= len(actualBody) {
						t.Errorf("Missing expected result at index %d", i)
						continue
					}

					actual := actualBody[i]
					if actual["owner_pubkey"] != expected["owner_pubkey"] {
						t.Errorf("Expected owner_pubkey %v, got %v", expected["owner_pubkey"], actual["owner_pubkey"])
					}
					if actual["total_bounties_completed"] == expected["total_bounties_completed"] {
						t.Errorf("Expected total_bounties_completed %v, got %v",
							expected["total_bounties_completed"], actual["total_bounties_completed"])
					}
					if actual["total_sats_earned"] == expected["total_sats_earned"] {
						t.Errorf("Expected total_sats_earned %v, got %v",
							expected["total_sats_earned"], actual["total_sats_earned"])
					}
				}
			}
		})
	}
}

func generateLargeUserSet(count int) []db.NewBounty {
	bounties := make([]db.NewBounty, count)
	for i := 0; i < count; i++ {
		bounties[i] = db.NewBounty{
			OwnerID:  fmt.Sprintf("user%d", i),
			Assignee: fmt.Sprintf("user%d", i),
			Price:    uint(1000 + i),
			Paid:     true,
			Type:     "coding",
			Title:    fmt.Sprintf("Test Bounty %d", i),
		}
	}
	return bounties
}

func TestGenerateBountyCardResponseAssigneeFields(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	workspace := db.Workspace{
		ID:          1,
		Uuid:        "test-workspace-uuid",
		Name:        "Test Workspace",
		Description: "Test Workspace Description",
		OwnerPubKey: "test-owner",
	}
	_, err := db.TestDB.CreateOrEditWorkspace(workspace)
	assert.NoError(t, err)

	assignee := db.Person{
		OwnerPubKey: "test-assignee",
		OwnerAlias:  "Test Assignee",
		Img:         "test-image-url",
	}
	_, err = db.TestDB.CreateOrEditPerson(assignee)
	assert.NoError(t, err)

	now := time.Now()

	testCases := []struct {
		name         string
		bounty       db.NewBounty
		expectedCard db.BountyCard
	}{
		{
			name: "Bounty with valid assignee",
			bounty: db.NewBounty{
				ID:            1,
				Title:         "Test Bounty",
				WorkspaceUuid: workspace.Uuid,
				Assignee:      "test-assignee",
				Created:       now.Unix(),
				OwnerID:       "test-owner",
				Type:          "coding",
			},
			expectedCard: db.BountyCard{
				BountyID:     1,
				Title:        "Test Bounty",
				AssigneePic:  "test-image-url",
				Assignee:     "test-assignee",
				AssigneeName: "Test Assignee",
			},
		},
		{
			name: "Bounty with no assignee",
			bounty: db.NewBounty{
				ID:            2,
				Title:         "Unassigned Bounty",
				WorkspaceUuid: workspace.Uuid,
				Created:       now.Unix(),
				OwnerID:       "test-owner",
				Type:          "coding",
			},
			expectedCard: db.BountyCard{
				BountyID:     2,
				Title:        "Unassigned Bounty",
				AssigneePic:  "",
				Assignee:     "",
				AssigneeName: "",
			},
		},
		{
			name: "Bounty with invalid assignee",
			bounty: db.NewBounty{
				ID:            3,
				Title:         "Invalid Assignee Bounty",
				WorkspaceUuid: workspace.Uuid,
				Assignee:      "non-existent-assignee",
				Created:       now.Unix(),
				OwnerID:       "test-owner",
				Type:          "coding",
			},
			expectedCard: db.BountyCard{
				BountyID:     3,
				Title:        "Invalid Assignee Bounty",
				AssigneePic:  "",
				Assignee:     "",
				AssigneeName: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db.TestDB.DeleteAllBounties()
			_, err := db.TestDB.CreateOrEditBounty(tc.bounty)
			assert.NoError(t, err)

			response := bHandler.GenerateBountyCardResponse([]db.NewBounty{tc.bounty})
			assert.Equal(t, 1, len(response))

			assert.Equal(t, tc.expectedCard.AssigneePic, response[0].AssigneePic)
			assert.Equal(t, tc.expectedCard.Assignee, response[0].Assignee)
			assert.Equal(t, tc.expectedCard.AssigneeName, response[0].AssigneeName)
			assert.Equal(t, tc.expectedCard.Title, response[0].Title)
			assert.Equal(t, tc.expectedCard.BountyID, response[0].BountyID)
		})
	}
}

func TestBountyCardResponsePerformance(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	workspace := db.Workspace{
		ID:          1,
		Uuid:        "test-workspace-uuid",
		Name:        "Test Workspace",
		Description: "Test Workspace Description",
		OwnerPubKey: "test-owner",
	}
	_, err := db.TestDB.CreateOrEditWorkspace(workspace)
	assert.NoError(t, err)

	assignee := db.Person{
		OwnerPubKey: "test-assignee",
		OwnerAlias:  "Test Assignee",
		Img:         "test-image-url",
	}
	_, err = db.TestDB.CreateOrEditPerson(assignee)
	assert.NoError(t, err)

	now := time.Now()
	bounties := make([]db.NewBounty, 100)
	for i := 0; i < 100; i++ {
		bounties[i] = db.NewBounty{
			ID:            uint(i + 1),
			Title:         fmt.Sprintf("Test Bounty %d", i),
			WorkspaceUuid: workspace.Uuid,
			Assignee:      "test-assignee",
			Created:       now.Unix(),
		}
	}

	start := time.Now()
	response := bHandler.GenerateBountyCardResponse(bounties)
	duration := time.Since(start)

	assert.Equal(t, 100, len(response))
	assert.Less(t, duration.Milliseconds(), int64(1000), "Response generation should take less than 1 second")

	assert.Equal(t, "Test Bounty 0", response[0].Title)
	assert.Equal(t, "test-assignee", response[0].Assignee)
	assert.Equal(t, "Test Assignee", response[0].AssigneeName)
	assert.Equal(t, "test-image-url", response[0].AssigneePic)

	assert.Equal(t, "Test Bounty 99", response[99].Title)
	assert.Equal(t, "test-assignee", response[99].Assignee)
	assert.Equal(t, "Test Assignee", response[99].AssigneeName)
	assert.Equal(t, "test-image-url", response[99].AssigneePic)
}

func TestBountyTiming(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	mockDB := dbMocks.NewDatabase(t)
	bHandler := NewBountyHandler(mockHttpClient, mockDB)

	t.Run("GetBountyTimingStats", func(t *testing.T) {
		t.Run("should return 400 for invalid bounty ID", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.GetBountyTimingStats)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "invalid")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodGet,
				"/timing",
				nil,
			)
			assert.NoError(t, err)

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("should return 500 when database fails", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.GetBountyTimingStats)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "1")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodGet,
				"/timing",
				nil,
			)
			assert.NoError(t, err)

			mockDB.On("GetBountyTiming", uint(1)).Return(nil, fmt.Errorf("database error")).Once()

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusInternalServerError, rr.Code)
			mockDB.AssertExpectations(t)
		})

		t.Run("should return timing stats successfully", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.GetBountyTimingStats)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "1")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodGet,
				"/timing",
				nil,
			)
			assert.NoError(t, err)

			now := time.Now()
			mockTiming := &db.BountyTiming{
				BountyID:             1,
				TotalWorkTimeSeconds: 3600,
				TotalDurationSeconds: 7200,
				TotalAttempts:        5,
				FirstAssignedAt:      &now,
				LastPoWAt:            &now,
				ClosedAt:             &now,
			}

			mockDB.On("GetBountyTiming", uint(1)).Return(mockTiming, nil).Once()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var response BountyTimingResponse
			err = json.NewDecoder(rr.Body).Decode(&response)
			assert.NoError(t, err)
			assert.Equal(t, mockTiming.TotalWorkTimeSeconds, response.TotalWorkTimeSeconds)
			assert.Equal(t, mockTiming.TotalAttempts, response.TotalAttempts)
			mockDB.AssertExpectations(t)
		})
	})

	t.Run("StartBountyTiming", func(t *testing.T) {
		t.Run("should return 400 for invalid bounty ID", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.StartBountyTiming)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "invalid")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodPut,
				"/timing/start",
				nil,
			)
			assert.NoError(t, err)

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("should return 500 when database fails", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.StartBountyTiming)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "1")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodPut,
				"/timing/start",
				nil,
			)
			assert.NoError(t, err)

			mockDB.On("StartBountyTiming", uint(1)).Return(fmt.Errorf("database error")).Once()

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusInternalServerError, rr.Code)
			mockDB.AssertExpectations(t)
		})

		t.Run("should start timing successfully", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.StartBountyTiming)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "1")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodPut,
				"/timing/start",
				nil,
			)
			assert.NoError(t, err)

			mockDB.On("StartBountyTiming", uint(1)).Return(nil).Once()

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			mockDB.AssertExpectations(t)
		})
	})

	t.Run("CloseBountyTiming", func(t *testing.T) {
		t.Run("should return 400 for invalid bounty ID", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.CloseBountyTiming)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "invalid")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodPut,
				"/timing/close",
				nil,
			)
			assert.NoError(t, err)

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("should return 500 when database fails", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.CloseBountyTiming)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "1")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodPut,
				"/timing/close",
				nil,
			)
			assert.NoError(t, err)

			mockDB.On("CloseBountyTiming", uint(1)).Return(fmt.Errorf("database error")).Once()

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusInternalServerError, rr.Code)
			mockDB.AssertExpectations(t)
		})

		t.Run("should close timing successfully", func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.CloseBountyTiming)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "1")
			req, err := http.NewRequestWithContext(
				context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
				http.MethodPut,
				"/timing/close",
				nil,
			)
			assert.NoError(t, err)

			mockDB.On("CloseBountyTiming", uint(1)).Return(nil).Once()

			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			mockDB.AssertExpectations(t)
		})
	})
}

func TestWorkspaceIsolation(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	// Create Workspace 1 and its components
	workspace1 := db.Workspace{
		ID:          1,
		Uuid:        "test-workspace-1",
		Name:        "Test Workspace 1",
		Description: "Test Workspace 1 Description",
		OwnerPubKey: "test-owner-1",
	}
	db.TestDB.CreateOrEditWorkspace(workspace1)

	feature1 := db.WorkspaceFeatures{
		Uuid:          "feature-1-uuid",
		Name:          "Feature One",
		WorkspaceUuid: workspace1.Uuid,
	}
	db.TestDB.CreateOrEditFeature(feature1)

	phase1 := db.FeaturePhase{
		Uuid:        "phase-1-uuid",
		Name:        "Feature One Phase",
		FeatureUuid: feature1.Uuid,
	}
	db.TestDB.CreateOrEditFeaturePhase(phase1)

	// Create Workspace 2 and its components
	workspace2 := db.Workspace{
		ID:          2,
		Uuid:        "test-workspace-2",
		Name:        "Test Workspace 2",
		Description: "Test Workspace 2 Description",
		OwnerPubKey: "test-owner-2",
	}
	db.TestDB.CreateOrEditWorkspace(workspace2)

	feature2 := db.WorkspaceFeatures{
		Uuid:          "feature-2-uuid",
		Name:          "Feature Two",
		WorkspaceUuid: workspace2.Uuid,
	}
	db.TestDB.CreateOrEditFeature(feature2)

	phase2 := db.FeaturePhase{
		Uuid:        "phase-2-uuid",
		Name:        "Feature Two Phase",
		FeatureUuid: feature2.Uuid,
	}
	db.TestDB.CreateOrEditFeaturePhase(phase2)

	// Create tickets for Workspace 1
	ticket1Group := uuid.New()
	ticket1 := db.Tickets{
		UUID:        uuid.New(),
		TicketGroup: &ticket1Group,
		Name:        "Ticket 1",
		FeatureUUID: feature1.Uuid,
		PhaseUUID:   phase1.Uuid,
		Status:      db.DraftTicket,
	}
	db.TestDB.CreateOrEditTicket(&ticket1)

	ticket2 := db.Tickets{
		UUID:        uuid.New(),
		TicketGroup: &ticket1Group,
		Name:        "Ticket 2",
		FeatureUUID: feature1.Uuid,
		PhaseUUID:   phase1.Uuid,
		Status:      db.DraftTicket,
		Version:     2,
	}
	db.TestDB.CreateOrEditTicket(&ticket2)

	// Create tickets for Workspace 2
	ticket3Group := uuid.New()
	ticket3 := db.Tickets{
		UUID:        uuid.New(),
		TicketGroup: &ticket3Group,
		Name:        "Phase 2 Ticket 1",
		FeatureUUID: feature2.Uuid,
		PhaseUUID:   phase2.Uuid,
		Status:      db.DraftTicket,
	}
	db.TestDB.CreateOrEditTicket(&ticket3)

	ticket4 := db.Tickets{
		UUID:        uuid.New(),
		TicketGroup: &ticket3Group,
		Name:        "Phase 2 Ticket 2",
		FeatureUUID: feature2.Uuid,
		PhaseUUID:   phase2.Uuid,
		Status:      db.DraftTicket,
	}
	db.TestDB.CreateOrEditTicket(&ticket4)

	t.Run("should only return workspace 1 tickets", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace1.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Should only contain tickets from workspace 1
		for _, card := range response {
			if card.TicketUUID != nil {
				assert.Equal(t, workspace1.Uuid, card.Features.WorkspaceUuid)
				assert.Contains(t, []string{"Ticket 1", "Ticket 2"}, card.Title)
				assert.NotContains(t, []string{"Phase 2 Ticket 1", "Phase 2 Ticket 2"}, card.Title)
			}
		}
	})

	t.Run("should only return workspace 2 tickets", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace2.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Should only contain tickets from workspace 2
		for _, card := range response {
			if card.TicketUUID != nil {
				assert.Equal(t, workspace2.Uuid, card.Features.WorkspaceUuid)
				assert.Contains(t, []string{"Phase 2 Ticket 1", "Phase 2 Ticket 2"}, card.Title)
				assert.NotContains(t, []string{"Ticket 1", "Ticket 2"}, card.Title)
			}
		}
	})

	t.Run("should handle non-existent workspace", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid=non-existent", nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, response, "Response should be empty for non-existent workspace")
	})

	t.Run("should handle workspace with no tickets", func(t *testing.T) {

		emptyWorkspace := db.Workspace{
			ID:          3,
			Uuid:        "empty-workspace",
			Name:        "Empty Workspace",
			Description: "Workspace with no tickets",
			OwnerPubKey: "test-owner-3",
		}
		db.TestDB.CreateOrEditWorkspace(emptyWorkspace)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+emptyWorkspace.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, response, "Response should be empty for workspace with no tickets")
	})

	t.Run("should handle workspace with only features but no tickets", func(t *testing.T) {

		featureOnlyWorkspace := db.Workspace{
			ID:          4,
			Uuid:        "feature-only-workspace",
			Name:        "Feature Only Workspace",
			Description: "Workspace with features but no tickets",
			OwnerPubKey: "test-owner-4",
		}
		db.TestDB.CreateOrEditWorkspace(featureOnlyWorkspace)

		featureOnly := db.WorkspaceFeatures{
			Uuid:          "feature-only-uuid",
			Name:          "Feature Without Tickets",
			WorkspaceUuid: featureOnlyWorkspace.Uuid,
		}
		db.TestDB.CreateOrEditFeature(featureOnly)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+featureOnlyWorkspace.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, response, "Response should be empty for workspace with only features")
	})

	t.Run("should verify the ticket lastest versions in workspace 1", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace1.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)

		var foundVersion2 bool
		for _, card := range response {
			if card.Title == "Ticket 2" {
				foundVersion2 = true
				break
			}
		}
		assert.True(t, foundVersion2, "Should find ticket with version 2")
	})

	t.Run("should verify feature and phase relationships", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace1.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)

		for _, card := range response {
			if card.TicketUUID != nil {

				assert.Equal(t, feature1.Uuid, card.Features.Uuid, "Feature UUID should match")
				assert.Equal(t, feature1.Name, card.Features.Name, "Feature name should match")

				assert.Equal(t, phase1.Uuid, card.Phase.Uuid, "Phase UUID should match")
				assert.Equal(t, phase1.Name, card.Phase.Name, "Phase name should match")
				assert.Equal(t, phase1.FeatureUuid, card.Phase.FeatureUuid, "Phase's feature UUID should match")
			}
		}
	})

	t.Run("should verify ticket status is draft", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.GetBountyCards)

		req, err := http.NewRequest(http.MethodGet, "/gobounties/bounty-cards?workspace_uuid="+workspace1.Uuid, nil)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)

		var response []db.BountyCard
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)

		for _, card := range response {
			if card.TicketUUID != nil {
				assert.Equal(t, db.StatusDraft, card.Status, "Ticket status should be draft")
			}
		}
	})
}

func TestDeleteBountyTiming(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	db.CleanTestData()

	bHandler := NewBountyHandler(http.DefaultClient, db.TestDB)

	testBounty := db.NewBounty{
		Type:          "coding",
		Title:         "Test Bounty for Timing Deletion",
		Description:   "Test bounty description",
		WorkspaceUuid: "test-workspace",
		OwnerID:       bountyOwner.OwnerPubKey,
		Created:       time.Now().Unix(),
	}

	createdBounty, err := db.TestDB.CreateOrEditBounty(testBounty)
	assert.NoError(t, err)

	_, err = db.TestDB.CreateBountyTiming(createdBounty.ID)
	assert.NoError(t, err)

	t.Run("should return 401 if no pubkey in context", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBountyTiming)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatUint(uint64(createdBounty.ID), 10))
		req, err := http.NewRequestWithContext(
			context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
			http.MethodDelete,
			"/timing",
			nil,
		)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 400 for invalid bounty ID", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBountyTiming)

		ctx := context.WithValue(context.Background(), auth.ContextKey, bountyOwner.OwnerPubKey)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "invalid")
		req, err := http.NewRequestWithContext(
			context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodDelete,
			"/timing",
			nil,
		)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return 404 when no timing record exists", func(t *testing.T) {

		db.CleanTestData()

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBountyTiming)

		ctx := context.WithValue(context.Background(), auth.ContextKey, bountyOwner.OwnerPubKey)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatUint(uint64(createdBounty.ID), 10))
		req, err := http.NewRequestWithContext(
			context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodDelete,
			"/timing",
			nil,
		)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("should successfully delete bounty timing", func(t *testing.T) {

		_, err := db.TestDB.CreateBountyTiming(createdBounty.ID)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(bHandler.DeleteBountyTiming)

		ctx := context.WithValue(context.Background(), auth.ContextKey, bountyOwner.OwnerPubKey)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", strconv.FormatUint(uint64(createdBounty.ID), 10))
		req, err := http.NewRequestWithContext(
			context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodDelete,
			"/timing",
			nil,
		)
		assert.NoError(t, err)

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		timing, err := db.TestDB.GetBountyTiming(createdBounty.ID)
		assert.Error(t, err)
		assert.Nil(t, timing)
	})
}

func TestInverseSearchBountyCards(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	mockHttpClient := mocks.NewHttpClient(t)
	bHandler := NewBountyHandler(mockHttpClient, db.TestDB)

	db.CleanTestData()

	workspace := db.Workspace{
		ID:          1,
		Uuid:        "test-workspace-uuid",
		Name:        "Test Workspace",
		Description: "Test Workspace Description",
		OwnerPubKey: "test-owner",
	}
	_, err := db.TestDB.CreateOrEditWorkspace(workspace)
	assert.NoError(t, err)

	phase := db.FeaturePhase{
		Uuid:        "test-phase-uuid",
		Name:        "Test Phase",
		FeatureUuid: "test-feature-uuid",
	}
	db.TestDB.CreateOrEditFeaturePhase(phase)

	feature := db.WorkspaceFeatures{
		Uuid:          "test-feature-uuid",
		Name:          "Test Feature",
		WorkspaceUuid: workspace.Uuid,
	}
	db.TestDB.CreateOrEditFeature(feature)

	now := time.Now()

	bounties := []db.NewBounty{
		{
			ID:            1,
			Type:          "coding",
			Title:         "Backend Task",
			Description:   "Backend development task",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Show:          true,
			Created:       now.Unix(),
			Updated:       &now,
			OwnerID:       "test-owner",
		},
		{
			ID:            2,
			Type:          "coding",
			Title:         "Frontend Task",
			Description:   "Frontend development task",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Show:          true,
			Created:       now.Unix(),
			Updated:       &now,
			OwnerID:       "test-owner",
		},
		{
			ID:            3,
			Type:          "coding",
			Title:         "Documentation Task",
			Description:   "Documentation writing task",
			WorkspaceUuid: workspace.Uuid,
			PhaseUuid:     phase.Uuid,
			Show:          true,
			Created:       now.Unix(),
			Updated:       &now,
			OwnerID:       "test-owner",
		},
	}

	for _, b := range bounties {
		_, err := db.TestDB.CreateOrEditBounty(b)
		assert.NoError(t, err)
	}

	testCases := []struct {
		name           string
		searchTerm     string
		inverseSearch  string
		expectedCount  int
		expectedTitles []string
	}{
		{
			name:           "inverse search excludes matching bounties",
			searchTerm:     "Frontend",
			inverseSearch:  "true",
			expectedCount:  2,
			expectedTitles: []string{"Backend Task", "Documentation Task"},
		},
		{
			name:           "regular search includes matching bounties",
			searchTerm:     "Frontend",
			inverseSearch:  "false",
			expectedCount:  1,
			expectedTitles: []string{"Frontend Task"},
		},
		{
			name:           "inverse search with common term",
			searchTerm:     "Task",
			inverseSearch:  "true",
			expectedCount:  0,
			expectedTitles: []string{},
		},
		{
			name:           "case insensitive inverse search",
			searchTerm:     "BACKEND",
			inverseSearch:  "true",
			expectedCount:  2,
			expectedTitles: []string{"Frontend Task", "Documentation Task"},
		},
		{
			name:           "empty search term returns all bounties",
			searchTerm:     "",
			inverseSearch:  "true",
			expectedCount:  3,
			expectedTitles: []string{"Backend Task", "Frontend Task", "Documentation Task"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(bHandler.GetBountyCards)

			url := fmt.Sprintf("/gobounties/bounty-cards?workspace_uuid=%s&search=%s&inverse_search=%s",
				workspace.Uuid, url.QueryEscape(tc.searchTerm), tc.inverseSearch)
			req, err := http.NewRequest(http.MethodGet, url, nil)
			assert.NoError(t, err)

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var response []db.BountyCard
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedCount, len(response),
				"Expected %d bounties but got %d", tc.expectedCount, len(response))

			responseTitles := make([]string, len(response))
			for i, card := range response {
				responseTitles[i] = card.Title
			}
			sort.Strings(responseTitles)
			sort.Strings(tc.expectedTitles)

			assert.Equal(t, tc.expectedTitles, responseTitles,
				"Expected titles do not match response titles")
		})
	}
}
