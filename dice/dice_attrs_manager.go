package dice

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ds "github.com/sealdice/dicescript"
	"go.uber.org/zap"

	"sealdice-core/dice/service"
	"sealdice-core/logger"
	"sealdice-core/model"
	"sealdice-core/utils/constant"
	"sealdice-core/utils/dboperator/engine"
)

type AttrsManager struct {
	db     engine.DatabaseOperator
	logger *zap.SugaredLogger
	cancel context.CancelFunc
	wg     sync.WaitGroup
	m      SyncMap[string, *AttributesItem]
}

func (am *AttrsManager) Stop() {
	logger.M().Info("结束数据库保存程序...")
	am.cancel()
	am.wg.Wait()
}

// LoadByCtx 获取当前角色，如有绑定，则获取绑定的角色，若无绑定，获取群内默认卡
func (am *AttrsManager) LoadByCtx(ctx *MsgContext) (*AttributesItem, error) {
	// 如果是兼容性测试环境，跳过绑定查询以避免不必要的数据库操作
	if ctx.IsCompatibilityTest {
		return am.LoadByIdDirect(ctx.Group.GroupID, ctx.Player.UserID)
	}
	return am.Load(ctx.Group.GroupID, ctx.Player.UserID)
}

func (am *AttrsManager) Load(groupId string, userId string) (*AttributesItem, error) {
	gid, id, err := am.resolveCharacterBinding(groupId, userId)
	if err != nil {
		return nil, err
	}

	// 2. 如果取不到，那么取用户在当前群的默认卡
	if id == "" {
		id = gid
	}

	return am.LoadById(id)
}

type characterBindingResolution struct {
	canonicalID string
	defaultID   string
	bindingID   string
	rowExists   bool
}

func (am *AttrsManager) equivalentGroupUserIDs(groupID string, userID string) (string, []string) {
	canonicalID := fmt.Sprintf("%s-%s", am.GIDConvert(groupID), am.UIDConvert(userID))
	groupIDs := officialQQIdentityOwnerCandidates(groupID)
	userIDs := officialQQIdentityOwnerCandidates(userID)
	if len(groupIDs) == 0 {
		groupIDs = []string{groupID}
	}
	if len(userIDs) == 0 {
		userIDs = []string{userID}
	}

	ids := make([]string, 0, len(groupIDs)*len(userIDs))
	seen := map[string]bool{}
	appendID := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	appendID(canonicalID)
	for _, candidateGroupID := range groupIDs {
		for _, candidateUserID := range userIDs {
			appendID(fmt.Sprintf("%s-%s", candidateGroupID, candidateUserID))
		}
	}
	return canonicalID, ids
}

func (am *AttrsManager) existingCharacterBinding(groupID string, userID string) (characterBindingResolution, error) {
	canonicalID, ids := am.equivalentGroupUserIDs(groupID, userID)
	resolution := characterBindingResolution{canonicalID: canonicalID, defaultID: canonicalID}
	bindings, err := service.AttrsGetBindingSheetIdsByGroupIds(am.db, ids)
	if err != nil {
		return resolution, err
	}
	for _, id := range ids {
		bindingID, exists := bindings[id]
		if !exists {
			continue
		}
		if !resolution.rowExists {
			resolution.defaultID = id
			resolution.rowExists = true
		}
		if bindingID != "" {
			resolution.bindingID = bindingID
			return resolution, nil
		}
	}
	return resolution, nil
}

// resolveCharacterBinding first searches native bindings stored under every
// verified historical representation of this same QQ user and group. Only a
// genuinely new group-user identity may inherit a unique active character.
func (am *AttrsManager) resolveCharacterBinding(groupID string, userID string) (string, string, error) {
	resolution, err := am.existingCharacterBinding(groupID, userID)
	if err != nil || resolution.bindingID != "" || resolution.rowExists || am.UIDConvert(userID) == userID {
		return resolution.defaultID, resolution.bindingID, err
	}

	inheritedID, err := am.findUniqueBoundCharacterID(userID)
	if err != nil {
		return resolution.defaultID, "", err
	}
	if inheritedID == "" {
		return resolution.defaultID, "", nil
	}
	if err := service.AttrsBindCharacter(am.db, inheritedID, resolution.canonicalID); err != nil {
		return resolution.defaultID, "", err
	}
	return resolution.canonicalID, inheritedID, nil
}

func (am *AttrsManager) findUniqueBoundCharacterID(userID string) (string, error) {
	items, err := am.GetCharacterList(userID)
	if err != nil {
		return "", err
	}
	foundID := ""
	for _, item := range items {
		if item == nil || item.BindingGroupsNum == 0 {
			continue
		}
		if foundID != "" && foundID != item.Id {
			return "", nil
		}
		foundID = item.Id
	}
	return foundID, nil
}

// InheritUniqueOfficialQQCharacterBinding is used immediately after an
// Official QQ identity is bound. It also handles group-user rows created while
// that OpenID was still unbound, which the passive resolver must preserve.
func (am *AttrsManager) InheritUniqueOfficialQQCharacterBinding(groupID string, officialUserID string) (string, bool, error) {
	canonicalUserID := am.UIDConvert(officialUserID)
	if canonicalUserID == officialUserID {
		return "", false, nil
	}
	resolution, err := am.existingCharacterBinding(groupID, officialUserID)
	if err != nil || resolution.bindingID != "" {
		return resolution.bindingID, false, err
	}
	bindingID, err := am.findUniqueBoundCharacterID(officialUserID)
	if err != nil || bindingID == "" {
		return "", false, err
	}
	if err := service.AttrsBindCharacter(am.db, bindingID, resolution.canonicalID); err != nil {
		return "", false, err
	}
	return bindingID, true, nil
}

// LoadByIdDirect 直接使用组合ID加载数据，跳过绑定查询（用于兼容性测试）
func (am *AttrsManager) LoadByIdDirect(groupId string, userId string) (*AttributesItem, error) {
	groupId = am.GIDConvert(groupId)
	userId = am.UIDConvert(userId)
	// 直接使用组合ID，跳过绑定查询
	id := fmt.Sprintf("%s-%s", groupId, userId)
	return am.LoadById(id)
}

func (am *AttrsManager) UIDConvert(userId string) string {
	return resolveOfficialQQIdentityAlias(userId)
}

func (am *AttrsManager) GIDConvert(groupId string) string {
	return resolveOfficialQQIdentityAlias(groupId)
}

func (am *AttrsManager) GetCharacterList(userId string) ([]*model.AttributesItemModel, error) {
	candidates := officialQQIdentityOwnerCandidates(userId)
	result := make([]*model.AttributesItemModel, 0)
	seen := map[string]bool{}
	for _, ownerID := range candidates {
		items, err := service.AttrsGetCharacterListByUserId(am.db, ownerID)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item == nil || seen[item.Id] {
				continue
			}
			seen[item.Id] = true
			result = append(result, item)
		}
	}
	return result, nil
}

func (am *AttrsManager) CharNew(userId string, name string, sheetType string) (*model.AttributesItemModel, error) {
	userId = am.UIDConvert(userId)
	dict := &ds.ValueMap{}
	// dict.Store("$sheetType", ds.NewStrVal(sheetType))
	json, err := ds.NewDictVal(dict).V().ToJSON()
	if err != nil {
		return nil, err
	}

	return service.AttrsNewItem(am.db, &model.AttributesItemModel{
		Name:      name,
		OwnerId:   userId,
		AttrsType: "character",
		SheetType: sheetType,
		Data:      json,
	})
}

func (am *AttrsManager) CharDelete(id string) error {
	if err := service.AttrsDeleteById(am.db, id); err != nil {
		return err
	}
	// 从缓存中删除
	am.m.Delete(id)
	return nil
}

// LoadById 数据加载，负责以下数据
// 1. 群内用户的默认卡(id格式为：群id:用户id)
// 2. 用户创建出的角色卡（指定id）
// 3. 群属性(id为群id)
// 4. 用户全局属性
func (am *AttrsManager) LoadById(id string) (*AttributesItem, error) {
	// 1. 如果当前有缓存，那么从缓存中返回。
	// 但是。。如果有人把这个对象一直持有呢？
	i, exists := am.m.Load(id)
	if exists && i != nil {
		return i, nil
	}

	// 2. 从新数据库加载
	data, err := service.AttrsGetById(am.db, id)
	if err == nil {
		if data.IsDataExists() {
			var v *ds.VMValue
			v, err = ds.VMValueFromJSON(data.Data)
			if err != nil {
				return nil, err
			}
			if dd, ok := v.ReadDictData(); ok {
				if dd.Dict == nil {
					dd.Dict = &ds.ValueMap{}
				}
				i = &AttributesItem{
					ID:           id,
					valueMap:     dd.Dict,
					Name:         data.Name,
					SheetType:    data.SheetType,
					LastUsedTime: time.Now().Unix(),
					IsSaved:      true,
				}
				am.m.Store(id, i)
				return i, nil
			} else {
				return nil, errors.New("角色数据类型不正确")
			}
		}
	}
	// 之前 else 是读不出时返回报错
	// return nil, err
	// 改为创建新数据集。因为遇到一个特别案例：因为clr前会读取当前角色数据，因为读不出来所以无法st clr
	// 从而永久卡死

	// 3. 创建一个新的
	// 注: 缺 created_at、updated_at、sheet_type、owner_id、is_hidden、nickname等各项
	// 可能需要ctx了
	now := time.Now().Unix()
	i = &AttributesItem{
		ID:               id,
		valueMap:         &ds.ValueMap{},
		LastModifiedTime: now,
		LastUsedTime:     now,
		IsSaved:          false,
	}
	am.m.Store(id, i)
	return i, nil
}

func (am *AttrsManager) Init(d *Dice) {
	am.db = d.DBOperator
	am.logger = d.Logger
	// 创建一个 context 用于取消 goroutine
	ctx, cancel := context.WithCancel(context.Background())
	// 启动后台定时任务
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// 检测到取消信号，执行最后一次保存后退出
				d.Logger.Info("正在执行最后一次数据保存...")
				if err := am.CheckForSave(); err != nil {
					d.Logger.Errorf("最终数据保存失败: %v", err)
				}
				return
			case <-ticker.C:
				// 定时执行保存和清理任务
				if err := am.CheckForSave(); err != nil {
					d.Logger.Errorf("数据库保存程序出错: %v", err)
				}
				if err := am.CheckAndFreeUnused(); err != nil {
					d.Logger.Errorf("数据库保存-清理程序出错: %v", err)
				}
			}
		}
	}()
	am.cancel = cancel
}

func (am *AttrsManager) CheckForSave() error {
	log := logger.M()
	if am.db == nil {
		// 尚未初始化
		return errors.New("数据库尚未初始化")
	}
	var resultList []*service.AttributesBatchUpsertModel
	prepareToSave := map[string]int{}
	am.m.Range(func(key string, value *AttributesItem) bool {
		if !value.IsSaved {
			saveModel, err := value.GetBatchSaveModel()
			if err != nil {
				// 打印日志
				log.Errorf("定期写入用户数据出错(获取批量保存模型): %v", err)
				return true
			}
			prepareToSave[key] = 1
			resultList = append(resultList, saveModel)
		}
		return true
	})
	// 整体落盘
	if len(resultList) == 0 {
		return nil
	}

	if err := service.AttrsPutsByIDBatch(am.db, resultList); err != nil {
		log.Errorf("定期写入用户数据出错(批量保存): %v", err)
		return err
	}
	for key := range prepareToSave {
		// 理应不存在这个数据没有的情况
		v, _ := am.m.Load(key)
		v.IsSaved = true
	}
	// 输出日志本次落盘了几个数据

	return nil
}

// CheckAndFreeUnused 此函数会被定期调用，释放最近不用的对象
func (am *AttrsManager) CheckAndFreeUnused() error {
	log := logger.M()
	db := am.db.GetDataDB(constant.WRITE)
	if db == nil {
		// 尚未初始化
		return errors.New("数据库尚未初始化")
	}

	prepareToFree := map[string]int{}
	currentTime := time.Now()
	var resultList []*service.AttributesBatchUpsertModel
	am.m.Range(func(key string, value *AttributesItem) bool {
		lastUsedTime := time.Unix(value.LastUsedTime, 0)
		lastModifiedTime := time.Unix(value.LastModifiedTime, 0)
		// 当且仅当上次修改时间超过10分钟，且上次使用时间超过10分钟的数据才会被释放。
		if currentTime.Sub(lastUsedTime) > 10*time.Minute && currentTime.Sub(lastModifiedTime) > 10*time.Minute {
			saveModel, err := value.GetBatchSaveModel()
			if err != nil {
				// 打印日志
				log.Errorf("定期清理用户数据出错(获取批量保存模型): %v", err)
				return true
			}
			prepareToFree[key] = 1
			resultList = append(resultList, saveModel)
		}
		return true
	})

	// 整体落盘
	if len(resultList) == 0 {
		return nil
	}

	if err := service.AttrsPutsByIDBatch(am.db, resultList); err != nil {
		log.Errorf("定期清理写入用户数据出错(批量保存): %v", err)
		return err
	}

	for key := range prepareToFree {
		// 理应不存在这个数据没有的情况
		v, ok := am.m.LoadAndDelete(key)
		if ok {
			v.IsSaved = true
		}
	}
	return nil
}

func (am *AttrsManager) CharBind(charId string, groupId string, userId string) error {
	canonicalID, ids := am.equivalentGroupUserIDs(groupId, userId)
	if charId != "" {
		return service.AttrsBindCharacter(am.db, charId, canonicalID)
	}

	bindings, err := service.AttrsGetBindingSheetIdsByGroupIds(am.db, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == canonicalID {
			continue
		}
		if _, exists := bindings[id]; exists {
			if err := service.AttrsBindCharacter(am.db, "", id); err != nil {
				return err
			}
		}
	}
	return service.AttrsBindCharacter(am.db, "", canonicalID)
}

// CharGetBindingId 获取当前群绑定的角色ID
func (am *AttrsManager) CharGetBindingId(groupId string, userId string) (string, error) {
	_, bindingID, err := am.resolveCharacterBinding(groupId, userId)
	return bindingID, err
}

func (am *AttrsManager) CharIdGetByName(userId string, name string) (string, error) {
	for _, ownerID := range officialQQIdentityOwnerCandidates(userId) {
		id, err := service.AttrsGetIdByUidAndName(am.db, ownerID, name)
		if err != nil {
			return "", err
		}
		if id != "" {
			return id, nil
		}
	}
	return "", nil
}

func (am *AttrsManager) CharCheckExists(userId string, name string) bool {
	id, _ := am.CharIdGetByName(userId, name)
	return id != ""
}

func (am *AttrsManager) CharGetBindingGroupIdList(id string) []string {
	all, err := service.AttrsCharGetBindingList(am.db, id)
	if err != nil {
		return []string{}
	}
	// 只要群号
	for i, v := range all {
		a, b, _ := UnpackGroupUserId(v)
		if b != "" {
			all[i] = a
		} else {
			all[i] = b
		}
	}
	return all
}

func (am *AttrsManager) CharUnbindAll(id string) []string {
	all := am.CharGetBindingGroupIdList(id)
	_, err := service.AttrsCharUnbindAll(am.db, id)
	if err != nil {
		return []string{}
	}
	return all
}

// AttributesItem 这是一个人物卡对象
type AttributesItem struct {
	ID               string
	valueMap         *ds.ValueMap // SyncMap[string, *ds.VMValue] 这种类型更好吗？我得确认一下js兼容性
	LastModifiedTime int64        // 上次修改时间
	LastUsedTime     int64        // 上次使用时间
	IsSaved          bool
	Name             string
	SheetType        string
}

func (i *AttributesItem) SaveToDB(db engine.DatabaseOperator) {
	// 使用事务写入
	rawData, err := ds.NewDictVal(i.valueMap).V().ToJSON()
	if err != nil {
		return
	}
	err = service.AttrsPutById(db, i.ID, rawData, i.Name, i.SheetType)
	if err != nil {
		logger.M().Error("保存数据失败", err.Error())
		return
	}
	i.IsSaved = true
}

func (i *AttributesItem) GetBatchSaveModel() (*service.AttributesBatchUpsertModel, error) {
	rawData, err := ds.NewDictVal(i.valueMap).V().ToJSON()
	if err != nil {
		return nil, err
	}
	return &service.AttributesBatchUpsertModel{
		Id:        i.ID,
		Data:      rawData,
		Name:      i.Name,
		SheetType: i.SheetType,
	}, nil
}

func (i *AttributesItem) Load(name string) *ds.VMValue {
	v, _ := i.valueMap.Load(name)
	i.LastUsedTime = time.Now().Unix()
	return v
}

func (i *AttributesItem) LoadX(name string) (*ds.VMValue, bool) {
	v, exists := i.valueMap.Load(name)
	i.LastUsedTime = time.Now().Unix()
	return v, exists
}

func (i *AttributesItem) Delete(name string) {
	i.valueMap.Delete(name)
	i.LastModifiedTime = time.Now().Unix()
}

func (i *AttributesItem) SetModified() {
	i.LastModifiedTime = time.Now().Unix()
	i.IsSaved = false
}

func (i *AttributesItem) Store(name string, value *ds.VMValue) {
	now := time.Now().Unix()
	i.valueMap.Store(name, value)
	i.LastModifiedTime = now
	i.LastUsedTime = now
	i.IsSaved = false
}

func (i *AttributesItem) Clear() int {
	size := i.valueMap.Length()
	i.valueMap.Clear()
	i.LastModifiedTime = time.Now().Unix()
	i.IsSaved = false
	return size
}

func (i *AttributesItem) ToArrayKeys() []*ds.VMValue {
	var items []*ds.VMValue
	i.valueMap.Range(func(key string, value *ds.VMValue) bool {
		items = append(items, ds.NewStrVal(key))
		return true
	})
	return items
}

func (i *AttributesItem) ToArrayValues() []*ds.VMValue {
	var items []*ds.VMValue
	i.valueMap.Range(func(key string, value *ds.VMValue) bool {
		items = append(items, value)
		return true
	})
	return items
}

func (i *AttributesItem) ToArrayItems() []*ds.VMValue {
	var items []*ds.VMValue
	i.valueMap.Range(func(key string, value *ds.VMValue) bool {
		items = append(
			items,
			ds.NewArrayVal(ds.NewStrVal(key), value),
		)
		return true
	})
	return items
}

func (i *AttributesItem) Range(f func(key string, value *ds.VMValue) bool) {
	i.valueMap.Range(f)
}

func (i *AttributesItem) SetSheetType(system string) {
	i.SheetType = system
	i.LastModifiedTime = time.Now().Unix()
	i.IsSaved = false
}

func (i *AttributesItem) Len() int {
	return i.valueMap.Length()
}
