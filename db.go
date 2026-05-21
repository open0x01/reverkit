package reverkit

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/iami317/hubur"
	"go.etcd.io/bbolt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var (
	httpEventBucketName = []byte("http")
	rmiEventBucketName  = []byte("rmi")
	metaBucketName      = []byte("meta")

	httpRespConfigBucketName = []byte("http")

	logBucketName        = []byte("reverse_log")
	groupBucketName      = []byte("reverse_group")
	respConfigBucketName = []byte("resp_config")
)

type DB struct {
	sync.Mutex
	path     string
	isTempDB bool
	*bbolt.DB
}

func newDB(dbFilePath string) (*DB, error) {
	reverseDB := &DB{}
	if dbFilePath == "" {
		reverseDB.isTempDB = true
		dbFilePath = filepath.Join(os.TempDir(), tempDBFilePrefix+hubur.RandStr(8))
		logger.Verbosef("reverse temp db path: %s", dbFilePath)
	}
	reverseDB.path = dbFilePath

	db, err := bbolt.Open(dbFilePath, 0600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		if err == bbolt.ErrTimeout {
			return nil, fmt.Errorf("open reverse database timeout, " +
				"this database can not be opened by multi process, " +
				"you can specify a new path in config file")
		} else {
			return nil, fmt.Errorf("failed to open reverse database, error %v", err)
		}
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{logBucketName, groupBucketName, respConfigBucketName} {
			_, err := tx.CreateBucketIfNotExists(name)
			if err != nil {
				return err
			}
		}
		for _, eventName := range [][]byte{httpEventBucketName, rmiEventBucketName, metaBucketName} {
			_, err := tx.Bucket(logBucketName).CreateBucketIfNotExists(eventName)
			if err != nil {
				return err
			}
		}

		for _, configName := range [][]byte{httpRespConfigBucketName} {
			_, err := tx.Bucket(respConfigBucketName).CreateBucketIfNotExists(configName)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	reverseDB.DB = db

	return reverseDB, err
}

func (d *DB) Close() {
	d.Lock()
	defer d.Unlock()
	if d.DB != nil {
		_ = d.DB.Close()
	}
	d.DB = nil
	if d.isTempDB {
		_ = os.Remove(d.path)
	}
}

func (d *DB) itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func (d *DB) incr(bucket *bbolt.Bucket, key []byte, value int) error {
	val, err := d.getInt(bucket, key)
	if err != nil {
		return err
	}
	return bucket.Put(key, []byte(strconv.Itoa(val+value)))
}

func (d *DB) getInt(bucket *bbolt.Bucket, key []byte) (int, error) {
	ret := 0
	originVal := bucket.Get(key)
	if originVal != nil {
		var err error
		ret, err = strconv.Atoi(string(originVal))
		if err != nil {
			return 0, err
		}
	}
	return ret, nil
}

func (d *DB) listEvent(lastID uint64, count uint, et eventType, action string) ([]*Event, int, error) {
	ret := make([]*Event, 0, count)
	total := 0
	var err error
	err = d.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(logBucketName).Bucket([]byte(et))
		cursor := bucket.Cursor()
		headNode := true
		if lastID != 0 {
			cursor.Seek(d.itob(lastID))
			headNode = false
		}

		for count != 0 {
			var k, v []byte
			if headNode {
				if action == actionNext {
					k, v = cursor.Last()
				} else {
					k, v = cursor.First()
				}
				headNode = false
			} else {
				if action == actionNext {
					k, v = cursor.Prev()
				} else {
					k, v = cursor.Next()
				}
			}
			if k == nil {
				break
			}
			event := &Event{}
			err = json.Unmarshal(v, event)
			if err != nil {
				return err
			}
			ret = append(ret, event)
			count -= 1
		}
		total, err = d.getInt(tx.Bucket(logBucketName).Bucket(metaBucketName), []byte(fmt.Sprintf("count_%s", et)))
		return err
	})
	return ret, total, err
}

func (d *DB) storeEvent(event *Event) error {
	d.Lock()
	defer d.Unlock()
	return d.Update(func(tx *bbolt.Tx) error {
		// reverse_log.http / reverse_log.dns bucket 中存放日志主体
		// event.id -> event 关系
		logBucket := tx.Bucket(logBucketName).Bucket([]byte(event.EventType))
		seq, _ := logBucket.NextSequence()
		idKey := d.itob(seq)
		event.ID = int64(seq)

		value, err := json.Marshal(event)
		if err != nil {
			return err
		}

		err = logBucket.Put(idKey, value)
		if err != nil {
			logger.Error(err)
			return err
		}

		err = d.incr(tx.Bucket(logBucketName).Bucket(metaBucketName), []byte(fmt.Sprintf("count_%s", event.EventType)), 1)
		if err != nil {
			logger.Error(err)
			return err
		}
		return nil
	})
}

func (d *DB) setHTTPResponse(config *HTTPResponseConfig) error {
	return d.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(respConfigBucketName).Bucket(httpRespConfigBucketName)
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}
		return b.Put([]byte(config.GroupID), data)
	})
}

func (d *DB) getHTTPResponse(groupID string) *HTTPResponseConfig {
	var ret *HTTPResponseConfig
	err := d.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(respConfigBucketName).Bucket(httpRespConfigBucketName)
		data := b.Get([]byte(groupID))
		if data == nil {
			return nil
		}
		ret = &HTTPResponseConfig{}
		return json.Unmarshal(data, ret)
	})
	if err != nil {
		logger.Error()
	}
	return ret
}
