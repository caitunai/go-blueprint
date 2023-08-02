package db

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	AccountId uint `gorm:"index" json:"account_id"`
}

func (u *User) Save(ctx context.Context) error {
	return db.WithContext(ctx).Save(u).Error
}

func GetUser(ctx context.Context, uid uint) *User {
	u := &User{}
	db.WithContext(ctx).Where("id", uid).First(u)
	return u
}

func RegisterUser(ctx context.Context, accountId uint) (*User, error) {
	u := &User{}
	err := DB().WithContext(ctx).Where("account_id = ?", accountId).First(u).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	u.AccountId = accountId
	err = u.Save(ctx)
	if err != nil {
		return nil, err
	}
	return u, nil
}
