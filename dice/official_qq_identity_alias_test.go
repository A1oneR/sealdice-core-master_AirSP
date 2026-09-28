package dice

import (
	"path/filepath"
	"reflect"
	"testing"

	ds "github.com/sealdice/dicescript"

	"sealdice-core/dice/service"
	"sealdice-core/model"
)

func TestOfficialQQIdentityOwnerCandidates(t *testing.T) {
	legacyID := "QQ:123456789"
	primaryID := "OpenQQ:3889204361-primary"
	secondaryID := "OpenQQ:3889204361-secondary"
	oldID := "OpenQQ-Member-T:123456789-old-group-primary"
	for _, officialID := range []string{primaryID, secondaryID, oldID} {
		RemoveOfficialQQIdentityAlias(officialID)
		if !RegisterOfficialQQIdentityAlias(officialID, legacyID) {
			t.Fatalf("failed to register test alias %q", officialID)
		}
		defer RemoveOfficialQQIdentityAlias(officialID)
	}

	want := []string{legacyID, oldID, primaryID, secondaryID}
	if got := officialQQIdentityOwnerCandidates(secondaryID); !reflect.DeepEqual(got, want) {
		t.Fatalf("owner candidates = %#v, want %#v", got, want)
	}
}

func TestAttrsManagerFindsCharacterCardsOwnedByOfficialQQAliases(t *testing.T) {
	legacyID := "QQ:234567890"
	primaryID := "OpenQQ:3889204361-primary-card-owner"
	secondaryID := "OpenQQ:3889204361-secondary-card-owner"
	oldID := "OpenQQ-Member-T:123456789-old-group-old-card-owner"
	for _, officialID := range []string{primaryID, secondaryID, oldID} {
		RemoveOfficialQQIdentityAlias(officialID)
		if !RegisterOfficialQQIdentityAlias(officialID, legacyID) {
			t.Fatalf("failed to register test alias %q", officialID)
		}
		defer RemoveOfficialQQIdentityAlias(officialID)
	}

	dbOperator, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "official-qq-alias-cards.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dbOperator.Close)
	if err := dbOperator.db.AutoMigrate(&model.AttributesItemModel{}); err != nil {
		t.Fatal(err)
	}

	cards := []model.AttributesItemModel{
		{Id: "canonical-card", AttrsType: "character", OwnerId: legacyID, Name: "QQ卡", CreatedAt: 1},
		{Id: "primary-card", AttrsType: "character", OwnerId: primaryID, Name: "首绑卡", CreatedAt: 2},
		{Id: "old-card", AttrsType: "character", OwnerId: oldID, Name: "老版卡", CreatedAt: 3},
		{Id: "hidden-card", AttrsType: "character", OwnerId: oldID, Name: "隐藏卡", IsHidden: true, CreatedAt: 4},
	}
	if err := dbOperator.db.Create(&cards).Error; err != nil {
		t.Fatal(err)
	}

	am := &AttrsManager{db: dbOperator}
	list, err := am.GetCharacterList(secondaryID)
	if err != nil {
		t.Fatal(err)
	}
	gotNames := make([]string, 0, len(list))
	for _, card := range list {
		gotNames = append(gotNames, card.Name)
	}
	wantNames := []string{"QQ卡", "老版卡", "首绑卡"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("character names = %#v, want %#v", gotNames, wantNames)
	}

	id, err := am.CharIdGetByName(secondaryID, "老版卡")
	if err != nil {
		t.Fatal(err)
	}
	if id != "old-card" {
		t.Fatalf("old character id = %q, want %q", id, "old-card")
	}
}

func TestAttrsManagerReadsHistoricalNativeBindingAcrossOfficialQQAliases(t *testing.T) {
	legacyUserID := "QQ:678901234"
	currentUserID := "OpenQQ:3889204361-current-native-binding"
	historicalUserID := "OpenQQ-Member-T:123456789-old-group-native-binding"
	legacyGroupID := "QQ-Group:678901234"
	currentGroupID := "OpenQQ-Group:3889204361-current-native-group"
	historicalGroupID := "OpenQQ-Group:123456789-historical-native-group"
	aliases := [][2]string{
		{currentUserID, legacyUserID},
		{historicalUserID, legacyUserID},
		{currentGroupID, legacyGroupID},
		{historicalGroupID, legacyGroupID},
	}
	for _, alias := range aliases {
		RemoveOfficialQQIdentityAlias(alias[0])
		if !RegisterOfficialQQIdentityAlias(alias[0], alias[1]) {
			t.Fatalf("failed to register test alias %q", alias[0])
		}
		defer RemoveOfficialQQIdentityAlias(alias[0])
	}

	dbOperator, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "official-qq-historical-native-binding.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dbOperator.Close)
	if err := dbOperator.db.AutoMigrate(&model.AttributesItemModel{}); err != nil {
		t.Fatal(err)
	}

	am := &AttrsManager{db: dbOperator}
	card, err := am.CharNew(legacyUserID, "原生旧卡", "coc7")
	if err != nil {
		t.Fatal(err)
	}
	cardAttrs, err := am.LoadById(card.Id)
	if err != nil {
		t.Fatal(err)
	}
	cardAttrs.Store("侦查", ds.NewIntVal(77))
	cardAttrs.SaveToDB(am.db)
	am.m.Delete(card.Id)
	canonicalGroupUserID := legacyGroupID + "-" + legacyUserID
	historicalGroupUserID := historicalGroupID + "-" + historicalUserID
	if err := service.AttrsBindCharacter(dbOperator, "", canonicalGroupUserID); err != nil {
		t.Fatal(err)
	}
	if err := service.AttrsBindCharacter(dbOperator, card.Id, historicalGroupUserID); err != nil {
		t.Fatal(err)
	}

	bindingID, err := am.CharGetBindingId(currentGroupID, currentUserID)
	if err != nil {
		t.Fatal(err)
	}
	if bindingID != card.Id {
		t.Fatalf("historical native binding = %q, want %q", bindingID, card.Id)
	}
	loaded, err := am.Load(currentGroupID, currentUserID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != card.Id || loaded.Name != "原生旧卡" {
		t.Fatalf("loaded historical native card = (%q, %q), want (%q, %q)", loaded.ID, loaded.Name, card.Id, "原生旧卡")
	}
	scout, exists := loaded.LoadX("侦查")
	if !exists || scout.MustReadInt() != 77 {
		t.Fatalf("historical native card attribute = (%v, %v), want (77, true)", scout, exists)
	}

	if err := am.CharBind("", currentGroupID, currentUserID); err != nil {
		t.Fatal(err)
	}
	bindingID, err = am.CharGetBindingId(currentGroupID, currentUserID)
	if err != nil {
		t.Fatal(err)
	}
	if bindingID != "" {
		t.Fatalf("historical native binding survived explicit unbind: %q", bindingID)
	}
	directBindingID, err := service.AttrsGetBindingSheetIdByGroupId(dbOperator, historicalGroupUserID)
	if err != nil {
		t.Fatal(err)
	}
	if directBindingID != "" {
		t.Fatalf("historical storage row was not unbound: %q", directBindingID)
	}
}

func TestAttrsManagerInheritsUniqueOfficialQQCharacterBindingAcrossGroups(t *testing.T) {
	legacyID := "QQ:345678901"
	primaryID := "OpenQQ:3889204361-primary-current-card"
	secondaryID := "OpenQQ:3889204361-secondary-current-card"
	for _, officialID := range []string{primaryID, secondaryID} {
		RemoveOfficialQQIdentityAlias(officialID)
		if !RegisterOfficialQQIdentityAlias(officialID, legacyID) {
			t.Fatalf("failed to register test alias %q", officialID)
		}
		defer RemoveOfficialQQIdentityAlias(officialID)
	}

	dbOperator, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "official-qq-current-card.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dbOperator.Close)
	if err := dbOperator.db.AutoMigrate(&model.AttributesItemModel{}); err != nil {
		t.Fatal(err)
	}

	am := &AttrsManager{db: dbOperator}
	card, err := am.CharNew(legacyID, "跨群调查员", "coc7")
	if err != nil {
		t.Fatal(err)
	}
	if err := am.CharBind(card.Id, "OpenQQ-Group:3889204361-primary-group", primaryID); err != nil {
		t.Fatal(err)
	}

	secondaryGroupID := "OpenQQ-Group:3889204361-secondary-group"
	bindingID, err := am.CharGetBindingId(secondaryGroupID, secondaryID)
	if err != nil {
		t.Fatal(err)
	}
	if bindingID != card.Id {
		t.Fatalf("secondary group binding = %q, want %q", bindingID, card.Id)
	}

	// A user may have spoken in another group before completing the secondary
	// binding, leaving an empty canonical group-user row behind. The explicit
	// post-binding migration must still attach the unique active card.
	preexistingGroupID := "OpenQQ-Group:3889204361-preexisting-group"
	if err := am.CharBind("", preexistingGroupID, secondaryID); err != nil {
		t.Fatal(err)
	}
	inheritedID, inherited, err := am.InheritUniqueOfficialQQCharacterBinding(preexistingGroupID, secondaryID)
	if err != nil {
		t.Fatal(err)
	}
	if !inherited || inheritedID != card.Id {
		t.Fatalf("post-binding inheritance = (%q, %v), want (%q, true)", inheritedID, inherited, card.Id)
	}

	loaded, err := am.Load(secondaryGroupID, secondaryID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != card.Id {
		t.Fatalf("secondary group loaded card = %q, want %q", loaded.ID, card.Id)
	}

	// An explicit unbind creates an existing row with an empty binding. It must
	// suppress inheritance on later commands in this group.
	if err := am.CharBind("", secondaryGroupID, secondaryID); err != nil {
		t.Fatal(err)
	}
	bindingID, err = am.CharGetBindingId(secondaryGroupID, secondaryID)
	if err != nil {
		t.Fatal(err)
	}
	if bindingID != "" {
		t.Fatalf("explicitly unbound group inherited card again: %q", bindingID)
	}
}

func TestAttrsManagerDoesNotGuessOfficialQQCharacterWhenSeveralAreActive(t *testing.T) {
	legacyID := "QQ:456789012"
	officialID := "OpenQQ:3889204361-multiple-current-cards"
	RemoveOfficialQQIdentityAlias(officialID)
	if !RegisterOfficialQQIdentityAlias(officialID, legacyID) {
		t.Fatal("failed to register test alias")
	}
	defer RemoveOfficialQQIdentityAlias(officialID)

	dbOperator, err := newMockDatabaseOperator(filepath.Join(t.TempDir(), "official-qq-ambiguous-cards.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dbOperator.Close)
	if err := dbOperator.db.AutoMigrate(&model.AttributesItemModel{}); err != nil {
		t.Fatal(err)
	}

	am := &AttrsManager{db: dbOperator}
	first, err := am.CharNew(legacyID, "调查员甲", "coc7")
	if err != nil {
		t.Fatal(err)
	}
	second, err := am.CharNew(legacyID, "调查员乙", "coc7")
	if err != nil {
		t.Fatal(err)
	}
	if err := am.CharBind(first.Id, "QQ-Group:111111111", legacyID); err != nil {
		t.Fatal(err)
	}
	if err := am.CharBind(second.Id, "QQ-Group:222222222", legacyID); err != nil {
		t.Fatal(err)
	}

	bindingID, err := am.CharGetBindingId("OpenQQ-Group:3889204361-new-group", officialID)
	if err != nil {
		t.Fatal(err)
	}
	if bindingID != "" {
		t.Fatalf("ambiguous active cards should require .pc tag, got %q", bindingID)
	}
}

func TestResolveOfficialQQIdentityAliasForPluginStorage(t *testing.T) {
	userOpenID := "OpenQQ:3889204361-plugin-storage-user"
	groupOpenID := "OpenQQ-Group:3889204361-plugin-storage-group"
	legacyUserID := "QQ:456789012"
	legacyGroupID := "QQ-Group:949517848"
	RemoveOfficialQQIdentityAlias(userOpenID)
	RemoveOfficialQQIdentityAlias(groupOpenID)
	t.Cleanup(func() {
		RemoveOfficialQQIdentityAlias(userOpenID)
		RemoveOfficialQQIdentityAlias(groupOpenID)
	})

	if !RegisterOfficialQQIdentityAlias(userOpenID, legacyUserID) {
		t.Fatal("failed to register user alias")
	}
	if !RegisterOfficialQQIdentityAlias(groupOpenID, legacyGroupID) {
		t.Fatal("failed to register group alias")
	}
	if got := ResolveOfficialQQIdentityAlias(userOpenID); got != legacyUserID {
		t.Fatalf("resolved user identity = %q, want %q", got, legacyUserID)
	}
	if got := ResolveOfficialQQIdentityAlias(groupOpenID); got != legacyGroupID {
		t.Fatalf("resolved group identity = %q, want %q", got, legacyGroupID)
	}
	if got := ResolveOfficialQQIdentityAlias(" OpenQQ:unbound "); got != "OpenQQ:unbound" {
		t.Fatalf("unbound identity = %q, want unchanged trimmed identity", got)
	}
}

func TestResolveOfficialQQGroupToGroupAlias(t *testing.T) {
	newGroupID := "OpenQQ-Group:1020304050-new-pure-group"
	oldGroupID := "OpenQQ-Group:3889204361-old-pure-group"
	RemoveOfficialQQIdentityAlias(newGroupID)
	t.Cleanup(func() {
		RemoveOfficialQQIdentityAlias(newGroupID)
	})

	if !RegisterOfficialQQIdentityAlias(newGroupID, oldGroupID) {
		t.Fatal("failed to register OpenQQ-Group to OpenQQ-Group alias")
	}
	if got := ResolveOfficialQQIdentityAlias(newGroupID); got != oldGroupID {
		t.Fatalf("resolved peer group alias = %q, want %q", got, oldGroupID)
	}
}
