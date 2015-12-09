package store

import (
	"errors"
	"github.com/boltdb/bolt"
)

const (
	// Permissions to use on the db file. This is only used if the
	// database file does not exist and needs to be created.
	dbFileMode = 0600
)

var (
	// Bucket for storing all Loaded plugin instance
	Plugin_instances_bucket = []byte("plugin_instances")

	// An error indicating a given key does not exist
	ErrNoSuchKey = errors.New("no such key exists")

	// An error indicating a given key does not exist
	ErrNoSuchBucket = errors.New("no such bucket exists")
)

// KVStore provides key/value storage
type KVStore struct {
	// conn is the handle to the db
	conn *bolt.DB

	// The path to the Bolt database file
	path string
}

// NewKVStore takes a file path and returns a new kvstore
func NewKVStore(path string) (*KVStore, error) {

	// Try to connect
	handle, err := bolt.Open(path, dbFileMode, nil)
	if err != nil {
		return nil, err
	}

	// Create new store
	store := &KVStore{
		conn: handle,
		path: path,
	}

	// Set up the buckets
	if err := store.initialize(); err != nil {
		store.Close()
		return nil, err
	}

	return store, nil
}

// initialize is used to set up all of the buckets.
func (b *KVStore) initialize() error {

	tx, err := b.conn.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.CreateBucketIfNotExists(Plugin_instances_bucket); err != nil {
		return err
	}
	return tx.Commit()
}

// Close is used to close the DB connection.
func (b *KVStore) Close() error {
	return b.conn.Close()
}

// Set is used to set a key/value
func (b *KVStore) Set(bucketToInsertIn, k, v []byte) error {
	tx, err := b.conn.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bucket := tx.Bucket(bucketToInsertIn)
	if err := bucket.Put(k, v); err != nil {
		return err
	}

	return tx.Commit()
}

// Get is used to retrieve a value from the k/v store by key
func (b *KVStore) Get(bucketToReadFrom, k []byte) ([]byte, error) {
	tx, err := b.conn.Begin(false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	bucket := tx.Bucket(bucketToReadFrom)
	val := bucket.Get(k)

	if val == nil {
		return nil, ErrNoSuchKey
	}
	return append([]byte{}, val...), nil
}

// GetAll is used to retrieve all the values stored in the bucket
func (b *KVStore) GetAll(bucketToReadFrom []byte, fn func(k, v []byte) error) error {
	tx, err := b.conn.Begin(false)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bucket := tx.Bucket(bucketToReadFrom)

	err = bucket.ForEach(fn)
	if err != nil {
		return err
	}

	return nil
}

// Del is used to delete a key from the k/v store by key
func (b *KVStore) Del(bucketToReadFrom, k []byte) error {
	tx, err := b.conn.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bucket := tx.Bucket(bucketToReadFrom)
	if bucket == nil {
		return ErrNoSuchBucket
	}
	err = bucket.Delete(k)
	if err != nil {
		return nil
	}

	return tx.Commit()
}
