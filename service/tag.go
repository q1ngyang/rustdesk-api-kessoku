package service

import (
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TagService struct {
}

func (s *TagService) Info(id uint) *model.Tag {
	p := &model.Tag{}
	DB.Where("id = ?", id).First(p)
	return p
}
func (s *TagService) InfoByUserIdAndNameAndCollectionId(userid uint, name string, cid uint) *model.Tag {
	p := &model.Tag{}
	DB.Where("user_id = ? and name = ? and collection_id = ?", userid, name, cid).First(p)
	return p
}

func (s *TagService) ListByUserId(userId uint) (res *model.TagList) {
	res = s.List(1, 1000, func(tx *gorm.DB) {
		tx.Where("user_id = ?", userId)
	})
	return
}
func (s *TagService) ListByUserIdAndCollectionId(userId, cid uint) (res *model.TagList) {
	res = s.List(1, 1000, func(tx *gorm.DB) {
		tx.Where("user_id = ? and collection_id = ?", userId, cid)
		tx.Order("name asc")
	})
	return
}
func (s *TagService) UpdateTags(userId uint, tags map[string]uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return s.updateTags(tx, userId, tags)
	})
}

func (s *TagService) updateTags(tx *gorm.DB, userId uint, tags map[string]uint) error {
	pending := make(map[string]uint, len(tags))
	for name, color := range tags {
		pending[name] = color
	}
	//先查询所有tag
	var allTags []*model.Tag
	if err := tx.Where("user_id = ? AND collection_id = 0", userId).Find(&allTags).Error; err != nil {
		return err
	}
	for _, t := range allTags {
		if _, ok := pending[t.Name]; !ok {
			//删除
			if err := tx.Delete(t).Error; err != nil {
				return err
			}
		} else {
			if pending[t.Name] != t.Color {
				//更新
				if err := tx.Model(t).UpdateColumn("color", pending[t.Name]).Error; err != nil {
					return err
				}
			}
			//移除
			delete(pending, t.Name)
		}
	}
	//新增
	for tag, color := range pending {
		t := &model.Tag{}
		t.Name = tag
		t.Color = color
		t.UserId = userId
		t.CollectionId = 0
		t.Collection = nil
		if err := tx.Omit(clause.Associations).Create(t).Error; err != nil {
			return err
		}
	}
	return nil
}

// InfoById 根据用户id取用户信息
func (s *TagService) InfoById(id uint) *model.Tag {
	u := &model.Tag{}
	DB.Where("id = ?", id).First(u)
	return u
}

func (s *TagService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.TagList) {
	res = &model.TagList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.Tag{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Tags)
	return
}

// Create 创建
func (s *TagService) Create(u *model.Tag) error {
	u.Collection = nil
	res := DB.Omit(clause.Associations).Create(u).Error
	return res
}
func (s *TagService) Delete(u *model.Tag) error {
	return DB.Delete(u).Error
}

// Update 更新
func (s *TagService) Update(u *model.Tag) error {
	u.Collection = nil
	return DB.Model(u).Select("*").Omit(clause.Associations, "created_at").Updates(u).Error
}
