package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/moby/locker"
	"github.com/threefoldtech/tf-pinning-service/pinning-api/models"
	"gorm.io/gorm"
)

type pins struct {
	db    *gorm.DB
	locks *locker.Locker
	mu    sync.Mutex
}

var locks = locker.New()

func GetPinsRepository(db *gorm.DB) PinsRepository {
	return &pins{
		db:    db,
		locks: locks,
	}
}

func (r *pins) InsertOrGet(ctx context.Context, user_id uint, pinStatus models.PinStatus) (models.PinStatus, error) {
	pin := PinDTO{}
	pin.FromEntity(pinStatus)

	pin.UserID = user_id
	uuid := pin.UUID
	pin.UUID = ""

	tx := r.db.Where("user_id = ? AND cid = ?", pin.UserID, pin.Cid).Attrs(PinDTO{UUID: uuid}).FirstOrCreate(&pin)
	if tx.Error != nil {
		return models.PinStatus{}, tx.Error
	}

	return pin.ToEntity(), nil
}

func (r *pins) Patch(ctx context.Context, user_id uint, id string, fields map[string]interface{}) error {
	tx := r.db.Model(&PinDTO{}).Where("uuid = ? AND user_id = ?", id, user_id).Updates(fields)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (r *pins) FindByID(ctx context.Context, user_id uint, id string) (models.PinStatus, error) {
	var pin PinDTO
	tx := r.db.First(&pin, "uuid = ? AND user_id = ?", id, user_id)
	if tx.RowsAffected == 0 {
		return models.PinStatus{}, errors.New("id not exists")
	}
	return pin.ToEntity(), nil
}

func (r *pins) Find(
	ctx context.Context,
	user_id uint,
	cids, statuses []string,
	name string,
	before, after time.Time,
	match string,
	limit int,
) (models.PinResults, error) {
	var pins []PinDTO

	queryDB := r.db.Model(PinDTO{})
	if len(cids) != 0 {
		queryDB = queryDB.Where("cid IN ?", cids)
	}
	if name != "" {
		switch models.TextMatchingStrategy(match) {
		case models.IEXACT:
			queryDB = queryDB.Where("name LIKE ?", name)
		case models.PARTIAL:
			fallthrough
		case models.IPARTIAL:
			name = fmt.Sprintf("%%%s%%", name)
			queryDB = queryDB.Where("name LIKE ?", name)
		// case string(models.EXACT):
		default:
			queryDB = queryDB.Where("name = ?", name)
		}
	}
	if len(statuses) != 0 {
		queryDB = queryDB.Where("status IN ?", statuses)
	}
	if user_id != 0 {
		queryDB = queryDB.Where("user_id = ?", user_id)
	}
	if !before.IsZero() {
		queryDB = queryDB.Where("created_at < ?", before.Unix())
	}

	if !after.IsZero() {
		queryDB = queryDB.Where("created_at > ?", after.Unix())
	}

	var count int64
	queryDB.Count(&count)
	tx := queryDB.Limit(limit).Order("created_at desc").Find(&pins)
	if tx.Error != nil {
		return models.PinResults{}, tx.Error
	}
	filterd_pins := []models.PinStatus{}
	for _, pin := range pins {
		pin_status := pin.ToEntity()
		filterd_pins = append(filterd_pins, pin_status)
	}

	return models.PinResults{Count: int32(count), Results: filterd_pins}, nil // TODO: check type PinResults.Count
}

func (r *pins) Delete(ctx context.Context, user_id uint, id string) error {
	tx := r.db.Where("uuid = ? AND user_id = ?", id, user_id).Delete(&PinDTO{})
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (r *pins) CIDRefrenceCount(ctx context.Context, cid string) (int64, error) {
	var count int64
	tx := r.db.Model(&PinDTO{}).Where("cid = ?", cid).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil

}

func (r *pins) ProcessByStatus(ctx context.Context, statuses []string) (chan []PinDTO, error) {
	queryDB := r.db
	var pins []PinDTO

	if len(statuses) != 0 {
		queryDB = queryDB.Where("status IN ?", statuses)
	}

	c := make(chan []PinDTO)
	go func() {
		result := queryDB.FindInBatches(&pins, 100, func(tx *gorm.DB, batch int) error {
			c <- pins
			//time.Sleep(time.Minute)
			//fmt.Println("tx.error: ", tx.Error)
			//fmt.Println("RowsAffected: ", tx.RowsAffected) // number of records in this batch

			//fmt.Println("Batch: ", batch) // Batch 1, 2, 3

			// returns error will stop future batches
			return nil
		})
		close(c)
		// TODO: log error
		fmt.Println("Error: ", result.Error) // returned error
		//fmt.Println(":", result.RowsAffected) // processed records count in all batches

	}()

	return c, nil
}

func (r *pins) LockByCID(cid string) {
	//fmt.Println("trying to acquire lock for: ", cid)
	r.locks.Lock(cid)
	//fmt.Println("lock acquired for: ", cid)

}

func (r *pins) UnlockByCID(cid string) {
	//fmt.Println("releasing lock for: ", cid)
	r.locks.Unlock(cid)
}

/* func (r *pins) Begin() *pins {
	tx := DB.Begin()
	return &pins{
		db: tx,
	}
}

func (r *pins) Rollback() {
	r.Rollback()
}

func (r *pins) Commit() {
	r.Commit()
}

func (r *pins) Lock() {
	r.mu.Lock()
}

func (r *pins) Unlock() {
	r.mu.Unlock()
} */
