//nolint:testpackage
package dice

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sealdice-core/dice/service"
	sealdiceLogger "sealdice-core/logger"
	"sealdice-core/model"
	"sealdice-core/utils/constant"
)

func newLogAliasTestDB(t *testing.T) *mockDatabaseOperator {
	t.Helper()

	mockDB, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("newMockDatabaseOperator: %v", err)
	}
	t.Cleanup(mockDB.Close)

	db := mockDB.GetLogDB(constant.WRITE)
	if err := db.AutoMigrate(&model.LogInfo{}, &model.LogOneItem{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return mockDB
}

func appendTestLog(t *testing.T, mockDB *mockDatabaseOperator, groupID, logName string) {
	t.Helper()

	ok := LogAppend(&MsgContext{Dice: &Dice{DBOperator: mockDB}}, groupID, 0, logName, &model.LogOneItem{
		Nickname: "tester",
		IMUserID: "user",
		Message:  "line",
	})
	if !ok {
		t.Fatalf("LogAppend failed for %q", logName)
	}
}

func TestBuildLogNameAliasIndexUsesWrappedKeysAndResolvesInput(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	groupID := "QQ-Group:1001"

	appendTestLog(t, mockDB, groupID, "normal-log")
	appendTestLog(t, mockDB, groupID, " spaced\nname ")

	index, err := buildLogNameAliasIndex(mockDB, groupID)
	if err != nil {
		t.Fatalf("buildLogNameAliasIndex: %v", err)
	}
	if len(index.entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(index.entries))
	}

	for _, entry := range index.entries {
		if len(entry.Key) < 8 {
			t.Fatalf("key %q shorter than 8 chars", entry.Key)
		}
		if entry.Name == "normal-log" {
			if got, ok := index.Resolve(entry.Key); !ok || got != entry.Name {
				t.Fatalf("Resolve(%q) = (%q, %v), want (%q, true)", entry.Key, got, ok, entry.Name)
			}
		}
	}

	if got := formatLogNameForDisplay(" spaced\nname "); got != strconv.Quote(" spaced\nname ") {
		t.Fatalf("formatLogNameForDisplay() = %q", got)
	}
	if got := formatLogNameListLine(logNameAliasEntry{Key: "deadbeef", Name: "normal-log"}); got != "- 【deadbeef】: normal-log" {
		t.Fatalf("formatLogNameListLine() = %q", got)
	}
}

func TestBuildLogNameAliasEntriesExtendsConflictingPrefixes(t *testing.T) {
	buckets := map[byte][]string{}
	for i := range 4096 {
		name := fmt.Sprintf("log-%d", i)
		hash := logNameHashHex(name)
		prefix := hash[0]
		buckets[prefix] = append(buckets[prefix], name)
		if len(buckets[prefix]) >= 2 {
			break
		}
	}

	var names []string
	for _, candidates := range buckets {
		if len(candidates) >= 2 {
			names = candidates[:2]
			break
		}
	}
	if len(names) != 2 {
		t.Fatal("failed to generate colliding 1-char hash prefixes")
	}

	entries, err := buildLogNameAliasEntries(names, 1)
	if err != nil {
		t.Fatalf("buildLogNameAliasEntries: %v", err)
	}

	keys := map[string]struct{}{}
	grew := false
	for _, entry := range entries {
		if _, exists := keys[entry.Key]; exists {
			t.Fatalf("duplicate key %q", entry.Key)
		}
		keys[entry.Key] = struct{}{}
		if len(entry.Key) > 1 {
			grew = true
		}
	}
	if !grew {
		t.Fatal("expected at least one conflicting prefix to grow beyond min length")
	}
}

func TestResolveLogNameForGroupPrefersExactNameBeforeAlias(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	groupID := "QQ-Group:1002"

	appendTestLog(t, mockDB, groupID, "abcdef12")
	appendTestLog(t, mockDB, groupID, "other-log")

	index, err := buildLogNameAliasIndex(mockDB, groupID)
	if err != nil {
		t.Fatalf("buildLogNameAliasIndex: %v", err)
	}

	var alias string
	for _, entry := range index.entries {
		if entry.Name == "other-log" {
			alias = entry.Key
			break
		}
	}
	if alias == "" {
		t.Fatal("expected alias for other-log")
	}
	if strings.EqualFold(alias, "abcdef12") {
		t.Fatal("test setup produced ambiguous exact-name collision")
	}

	got, err := resolveLogNameForGroup(mockDB, groupID, "abcdef12")
	if err != nil {
		t.Fatalf("resolveLogNameForGroup exact: %v", err)
	}
	if got != "abcdef12" {
		t.Fatalf("resolve exact = %q, want %q", got, "abcdef12")
	}
}

func TestLogSendToBackendAcceptsAliasKey(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	groupID := "QQ-Group:1003"
	logName := "special name"
	appendTestLog(t, mockDB, groupID, logName)

	index, err := buildLogNameAliasIndex(mockDB, groupID)
	if err != nil {
		t.Fatalf("buildLogNameAliasIndex: %v", err)
	}
	var alias string
	for _, entry := range index.entries {
		if entry.Name == logName {
			alias = entry.Key
			break
		}
	}
	if alias == "" {
		t.Fatal("expected alias for special name")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dice/api/log" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if parseErr := r.ParseMultipartForm(1 << 20); parseErr != nil {
			t.Fatalf("ParseMultipartForm: %v", parseErr)
		}
		if got := r.FormValue("name"); got != logName {
			t.Fatalf("uploaded name = %q, want %q", got, logName)
		}
		_, _ = w.Write([]byte(`{"url":"https://example.com/log"}`))
	}))
	defer server.Close()

	oldBackends := BackendUrls
	BackendUrls = []string{server.URL}
	t.Cleanup(func() { BackendUrls = oldBackends })

	ctx := &MsgContext{
		Dice: &Dice{
			BaseConfig: BaseConfig{DataDir: t.TempDir()},
			DBOperator: mockDB,
			Logger:     sealdiceLogger.M(),
		},
		EndPoint: &EndPointInfo{EndPointInfoBase: EndPointInfoBase{UserID: "UI:1000"}},
	}

	unofficial, url, notice, err := LogSendToBackend(ctx, groupID, alias)
	if err != nil {
		t.Fatalf("LogSendToBackend: %v", err)
	}
	if unofficial {
		t.Fatal("expected official backend path")
	}
	if notice != "" {
		t.Fatalf("unexpected notice %q", notice)
	}
	if url != "https://example.com/log" {
		t.Fatalf("url = %q", url)
	}
}

func TestEnsureGroupLogStateBackfillsLegacyLogID(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	groupID := "QQ-Group:2001"
	logName := "legacy-log"

	ctx := &MsgContext{
		Dice: &Dice{DBOperator: mockDB},
		Group: &GroupInfo{
			GroupID:    groupID,
			LogCurName: logName,
			LogOn:      true,
		},
	}

	logState := ensureGroupLogState(ctx, ctx.Group)
	if !logState.On {
		t.Fatal("expected legacy log state to stay on")
	}
	if logState.Name != logName {
		t.Fatalf("logState.Name = %q, want %q", logState.Name, logName)
	}
	if logState.ID == 0 {
		t.Fatal("expected legacy log state to backfill non-zero log id")
	}
	if got := ctx.Group.GetLogState().ID; got != logState.ID {
		t.Fatalf("group LogCurID = %d, want %d", got, logState.ID)
	}
}

func TestBuildLogNameAliasIndexUsesBoundOfficialQQGroup(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	officialGroupID := "OpenQQ-Group:synthetic-log-list"
	legacyGroupID := "QQ-Group:810001"
	logName := "legacy-campaign"

	if !service.LogAppend(mockDB, legacyGroupID, logName, &model.LogOneItem{Message: "old line"}) {
		t.Fatal("failed to seed legacy log")
	}
	if !RegisterOfficialQQIdentityAlias(officialGroupID, legacyGroupID) {
		t.Fatal("failed to register synthetic group alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(officialGroupID) })

	index, err := buildLogNameAliasIndex(mockDB, officialGroupID)
	if err != nil {
		t.Fatalf("buildLogNameAliasIndex: %v", err)
	}
	if len(index.entries) != 1 || index.entries[0].Name != logName {
		t.Fatalf("entries = %#v, want legacy log %q", index.entries, logName)
	}
}

func TestBoundOfficialQQGroupUsesLegacyLogForNativeOperations(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	officialGroupID := "OpenQQ-Group:synthetic-log-operations"
	legacyGroupID := "QQ-Group:810002"
	logName := "continued-campaign"
	rawMsgID := "synthetic-message-id"

	logID, err := service.LogGetOrCreate(mockDB, legacyGroupID, logName)
	if err != nil {
		t.Fatalf("seed legacy log: %v", err)
	}
	if !RegisterOfficialQQIdentityAlias(officialGroupID, legacyGroupID) {
		t.Fatal("failed to register synthetic group alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(officialGroupID) })

	ctx := &MsgContext{Dice: &Dice{DBOperator: mockDB, Logger: sealdiceLogger.M()}}
	if !LogAppend(ctx, officialGroupID, logID, logName, &model.LogOneItem{
		Message:  "continued line",
		RawMsgID: rawMsgID,
	}) {
		t.Fatal("LogAppend through official group failed")
	}

	lines, err := service.LogGetAllLines(mockDB, legacyGroupID, logName)
	if err != nil || len(lines) != 1 {
		t.Fatalf("legacy lines after append = %d, err = %v", len(lines), err)
	}
	var storedGroupID string
	if err := mockDB.GetLogDB(constant.READ).Model(&model.LogOneItem{}).
		Where("log_id = ?", logID).
		Pluck("group_id", &storedGroupID).Error; err != nil {
		t.Fatalf("read appended item group: %v", err)
	}
	if storedGroupID != legacyGroupID {
		t.Fatalf("appended item group = %q, want legacy storage group", storedGroupID)
	}
	if names, err := service.LogGetList(mockDB, officialGroupID); err != nil || len(names) != 0 {
		t.Fatalf("official storage unexpectedly contains logs: %v, err = %v", names, err)
	}

	logFile, _, err := GetLogTxt(ctx, officialGroupID, logName, "bound-log-test")
	if err != nil {
		t.Fatalf("GetLogTxt through official group: %v", err)
	}
	defer os.Remove(logFile)
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read exported log: %v", err)
	}
	if !strings.Contains(string(content), "continued line") {
		t.Fatalf("export does not contain legacy log line: %q", content)
	}

	if !LogEditByID(ctx, officialGroupID, "edited line", rawMsgID) {
		t.Fatal("LogEditByID through official group failed")
	}
	lines, err = service.LogGetAllLines(mockDB, legacyGroupID, logName)
	if err != nil || len(lines) != 1 || lines[0].Message != "edited line" {
		t.Fatalf("legacy line after edit = %#v, err = %v", lines, err)
	}

	if !LogDeleteByID(ctx, officialGroupID, rawMsgID) {
		t.Fatal("LogDeleteByID through official group failed")
	}
	lines, err = service.LogGetAllLines(mockDB, legacyGroupID, logName)
	if err != nil || len(lines) != 0 {
		t.Fatalf("legacy lines after delete = %d, err = %v", len(lines), err)
	}
}

func TestEnsureGroupLogStateRebindsPreexistingOfficialLogID(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	officialGroupID := "OpenQQ-Group:synthetic-log-state"
	legacyGroupID := "QQ-Group:810004"
	logName := "rebound-campaign"

	officialLogID, err := service.LogGetOrCreate(mockDB, officialGroupID, logName)
	if err != nil {
		t.Fatalf("seed official log: %v", err)
	}
	if !RegisterOfficialQQIdentityAlias(officialGroupID, legacyGroupID) {
		t.Fatal("failed to register synthetic group alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(officialGroupID) })
	legacyLogID, err := service.LogGetOrCreate(mockDB, legacyGroupID, logName)
	if err != nil {
		t.Fatalf("seed legacy log: %v", err)
	}

	ctx := &MsgContext{
		Dice:  &Dice{DBOperator: mockDB, Logger: sealdiceLogger.M()},
		Group: &GroupInfo{GroupID: officialGroupID, LogCurName: logName, LogCurID: officialLogID, LogOn: true},
	}
	state := ensureGroupLogState(ctx, ctx.Group)
	if state.ID != legacyLogID {
		t.Fatalf("reconciled log ID = %d, want legacy ID %d", state.ID, legacyLogID)
	}
}

func TestResolveLogStorageGroupIDLeavesUnboundGroupUnchanged(t *testing.T) {
	groupID := "QQ-Group:810003"
	if got := resolveLogStorageGroupID(groupID); got != groupID {
		t.Fatalf("resolveLogStorageGroupID(%q) = %q", groupID, got)
	}
}

func TestResolveOfficialQQLogIdentity(t *testing.T) {
	officialUser := "OpenQQ:3889204361-USER1"
	bareUser := "3889204361-USER1"
	legacyUser := "QQ:1120934969"
	officialGroup := "OpenQQ-Group:3889204361-GRP1"
	legacyGroup := "QQ-Group:88990011"

	if !RegisterOfficialQQIdentityAlias(officialUser, legacyUser) {
		t.Fatal("failed to register official user alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(officialUser) })

	if !RegisterOfficialQQIdentityAlias(officialGroup, legacyGroup) {
		t.Fatal("failed to register official group alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(officialGroup) })

	// Case 1: Full uniformID
	u, im := ResolveOfficialQQLogIdentity(officialUser, bareUser)
	if u != legacyUser || im != "1120934969" {
		t.Fatalf("ResolveOfficialQQLogIdentity(%q, %q) = (%q, %q), want (%q, %q)", officialUser, bareUser, u, im, legacyUser, "1120934969")
	}

	// Case 2: Bare imUserID without uniformID
	u, im = ResolveOfficialQQLogIdentity("", bareUser)
	if u != legacyUser || im != "1120934969" {
		t.Fatalf("ResolveOfficialQQLogIdentity('', %q) = (%q, %q), want (%q, %q)", bareUser, u, im, legacyUser, "1120934969")
	}

	// Case 3: Group identity
	u, im = ResolveOfficialQQLogIdentity(officialGroup, "3889204361-GRP1")
	if u != legacyGroup || im != "88990011" {
		t.Fatalf("ResolveOfficialQQLogIdentity(%q) = (%q, %q), want (%q, %q)", officialGroup, u, im, legacyGroup, "88990011")
	}

	// Case 4: Unbound OpenQQ
	u, im = ResolveOfficialQQLogIdentity("OpenQQ:unknown-user", "unknown-user")
	if u != "OpenQQ:unknown-user" || im != "unknown-user" {
		t.Fatalf("ResolveOfficialQQLogIdentity unbound = (%q, %q), want original", u, im)
	}

	// Case 5: Native numeric QQ
	u, im = ResolveOfficialQQLogIdentity("QQ:123456", "123456")
	if u != "QQ:123456" || im != "123456" {
		t.Fatalf("ResolveOfficialQQLogIdentity native QQ = (%q, %q), want original", u, im)
	}
}

func TestLogSendToBackendResolvesBoundIdentitiesInExport(t *testing.T) {
	mockDB := newLogAliasTestDB(t)
	officialGroupID := "OpenQQ-Group:synthetic-export-grp"
	legacyGroupID := "QQ-Group:99887766"
	officialUserID := "OpenQQ:synthetic-user-1"
	bareUserID := "synthetic-user-1"
	legacyUserID := "QQ:1120934969"
	logName := "export-bind-test"

	if !RegisterOfficialQQIdentityAlias(officialGroupID, legacyGroupID) {
		t.Fatal("failed to register official group alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(officialGroupID) })

	if !RegisterOfficialQQIdentityAlias(officialUserID, legacyUserID) {
		t.Fatal("failed to register official user alias")
	}
	t.Cleanup(func() { RemoveOfficialQQIdentityAlias(officialUserID) })

	logID, err := service.LogGetOrCreate(mockDB, legacyGroupID, logName)
	if err != nil {
		t.Fatalf("LogGetOrCreate failed: %v", err)
	}

	ctx := &MsgContext{
		Dice: &Dice{
			BaseConfig: BaseConfig{DataDir: t.TempDir()},
			DBOperator: mockDB,
			Logger:     sealdiceLogger.M(),
		},
		EndPoint: &EndPointInfo{EndPointInfoBase: EndPointInfoBase{UserID: officialUserID}},
	}

	ok := LogAppend(ctx, legacyGroupID, logID, logName, &model.LogOneItem{
		Nickname:  "Tester",
		UniformID: officialUserID,
		IMUserID:  bareUserID,
		Message:   "binding replacement message line",
		Time:      1700000000,
	})
	if !ok {
		t.Fatal("LogAppend failed")
	}

	var capturedBody []byte
	var capturedUniformID string
	var serverCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		if r.URL.Path != "/dice/api/log" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if parseErr := r.ParseMultipartForm(10 << 20); parseErr != nil {
			t.Fatalf("ParseMultipartForm: %v", parseErr)
		}
		capturedUniformID = r.FormValue("uniform_id")
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile failed: %v", err)
		}
		defer file.Close()
		capturedBody, _ = io.ReadAll(file)
		_, _ = w.Write([]byte(`{"url":"https://example.com/log-replaced"}`))
	}))
	defer server.Close()

	oldBackends := BackendUrls
	BackendUrls = []string{server.URL}
	t.Cleanup(func() { BackendUrls = oldBackends })

	unofficial, url, notice, err := LogSendToBackend(ctx, officialGroupID, logName)
	if err != nil {
		t.Fatalf("LogSendToBackend failed: %v", err)
	}
	if unofficial || url != "https://example.com/log-replaced" {
		t.Fatalf("unexpected result: unofficial=%v, url=%q, notice=%q", unofficial, url, notice)
	}
	if capturedUniformID != legacyUserID {
		t.Fatalf("captured uniform_id = %q, want legacy %q", capturedUniformID, legacyUserID)
	}

	// Decompress zlib payload
	zr, err := zlib.NewReader(bytes.NewReader(capturedBody))
	if err != nil {
		t.Fatalf("zlib.NewReader failed: %v", err)
	}
	uncompressed, err := io.ReadAll(zr)
	zr.Close()
	if err != nil {
		t.Fatalf("read uncompressed data failed: %v", err)
	}

	// Payload is the zip file (written by formatAndBackup)
	zipReader, err := zip.NewReader(bytes.NewReader(uncompressed), int64(len(uncompressed)))
	if err == nil {
		for _, f := range zipReader.File {
			rc, openErr := f.Open()
			if openErr == nil {
				content, _ := io.ReadAll(rc)
				rc.Close()
				contentStr := string(content)
				if f.Name == "log.txt" {
					if !strings.Contains(contentStr, "Tester(1120934969)") {
						t.Fatalf("zip log.txt missing Tester(1120934969): %s", contentStr)
					}
					if strings.Contains(contentStr, "synthetic-user-1") {
						t.Fatalf("zip log.txt still contains synthetic-user-1: %s", contentStr)
					}
				}
				if f.Name == "log.json" {
					if !strings.Contains(contentStr, `"im_user_id":"1120934969"`) && !strings.Contains(contentStr, `"im_user_id": "1120934969"`) {
						t.Fatalf("zip log.json missing im_user_id 1120934969: %s", contentStr)
					}
					if strings.Contains(contentStr, "synthetic-user-1") {
						t.Fatalf("zip log.json still contains synthetic-user-1: %s", contentStr)
					}
				}
			}
		}
	} else {
		// Fallback check on raw uncompressed string
		uncompressedStr := string(uncompressed)
		if !strings.Contains(uncompressedStr, "1120934969") {
			t.Fatalf("exported log payload missing resolved QQ 1120934969: %s", uncompressedStr)
		}
		if strings.Contains(uncompressedStr, "synthetic-user-1") {
			t.Fatalf("exported log payload still contains unreplaced OpenID synthetic-user-1: %s", uncompressedStr)
		}
	}

	// Verify GetLogTxt also replaces identity
	txtFile, _, err := GetLogTxt(ctx, officialGroupID, logName, "test-export")
	if err != nil {
		t.Fatalf("GetLogTxt failed: %v", err)
	}
	defer os.Remove(txtFile)
	txtContent, err := os.ReadFile(txtFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(txtContent), "Tester(1120934969)") {
		t.Fatalf("GetLogTxt output missing 'Tester(1120934969)': %s", string(txtContent))
	}
	if strings.Contains(string(txtContent), "synthetic-user-1") {
		t.Fatalf("GetLogTxt output still contains 'synthetic-user-1': %s", string(txtContent))
	}
}
