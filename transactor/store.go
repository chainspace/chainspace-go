package transactor // import "chainspace.io/prototype/transactor"

import (
	"errors"

	"github.com/dgraph-io/badger"
)

type keyType byte

const (
	keyTypeObjectValue keyType = iota
	keyTypeObjectStatus
	keyTypeCommittedTxn
	keyTypeSeenTxn
	keyFinishedTxn
)

var (
	ErrTransactionAlreadyExists    = errors.New("Transaction already exists")
	ErrTransactionAlreadyCommitted = errors.New("Transaction already committed")
	ErrObjectAlreadyExists         = errors.New("Output object already exists")
	ErrObjectAlreadyInactive       = errors.New("Object already in INACTIVE state")
	ErrObjectCannotBeLocked        = errors.New("Object cannot be locked (already INACTIVE or LOCKED)")
	ErrObjectCannotBeUnlocked      = errors.New("Object cannot be unlocked")
)

func makeKey(ty keyType, key []byte) []byte {
	out := make([]byte, len(key)+1)
	out[0] = byte(ty)
	copy(out[1:], key)
	return out
}

func finishedTxnKey(key []byte) []byte {
	return makeKey(keyFinishedTxn, key)
}

func objectKey(key []byte) []byte {
	return makeKey(keyTypeObjectValue, key)
}

func objectStatusKey(key []byte) []byte {
	return makeKey(keyTypeObjectStatus, key)
}

func committedTxnKey(key []byte) []byte {
	return makeKey(keyTypeCommittedTxn, key)
}

func seenTxnKey(key []byte) []byte {
	return makeKey(keyTypeSeenTxn, key)
}

func setObjectsInactive(txn *badger.Txn, objkeys [][]byte) error {
	for _, objkey := range objkeys {
		key := objectStatusKey(objkey)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		if ObjectStatus(val[0]) == ObjectStatus_INACTIVE {
			return ErrObjectAlreadyInactive
		}
		err = txn.Set(key, []byte{byte(ObjectStatus_INACTIVE)})
		if err != nil {
			return err
		}
	}
	return nil
}

func createObjects(txn *badger.Txn, objs []*Object) error {
	for _, obj := range objs {
		objkey := objectKey(obj.Key)
		_, err := txn.Get(objkey)
		if err == nil {
			return ErrObjectAlreadyExists
		}
		err = txn.Set(objkey, obj.Value)
		if err != nil {
			return err
		}
		objstatuskey := objectStatusKey(obj.Key)
		_, err = txn.Get(objstatuskey)
		if err == nil {
			return ErrObjectAlreadyExists
		}
		err = txn.Set(objstatuskey, []byte{byte(ObjectStatus_ACTIVE)})
		if err != nil {
			return err
		}
	}
	return nil
}

func commitTransaction(txn *badger.Txn, txnkey []byte) error {
	key := committedTxnKey(txnkey)
	item, err := txn.Get(key)
	if err != nil {
		return err
	}
	val, err := item.Value()
	if err != nil {
		return err
	}
	if val[0] == 1 { // tx already committed
		return ErrTransactionAlreadyCommitted
	}
	txn.Set(key, []byte{1})
	return nil
}

// CommitTransaction will in the same transaction move all object from their current state to
// inactive, creates output objects, then update the transaction commit status.
// if any operation is not possible, everything is rollback and an error is returned.
func CommitTransaction(store *badger.DB, txnkey []byte, inobjkeys [][]byte, objs []*Object) error {
	return store.Update(func(tx *badger.Txn) error {
		if err := setObjectsInactive(tx, inobjkeys); err != nil {
			return err
		}
		if err := createObjects(tx, objs); err != nil {
			return err
		}
		return commitTransaction(tx, txnkey)
	})
}

// LockObjects perform a lock on all the objects from the keys slice. If one+ object is already
// locked or inactive this action is rolled back and an error is returned
func LockObjects(store *badger.DB, objkeys [][]byte) error {
	return store.Update(func(txn *badger.Txn) error {
		for _, objkey := range objkeys {
			key := objectStatusKey(objkey)
			item, err := txn.Get(key)
			if err != nil {
				return err
			}
			val, err := item.Value()
			if err != nil {
				return err
			}
			if ObjectStatus(val[0]) != ObjectStatus_ACTIVE {
				return ErrObjectCannotBeLocked
			}
			err = txn.Set(key, []byte{byte(ObjectStatus_LOCKED)})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// UnlockObject unlock all object coresponding to the objects keys. If one+ object is inactive
// all operations are rolled back and an error is returned
func UnlockObjects(store *badger.DB, objkeys [][]byte) error {
	return store.Update(func(txn *badger.Txn) error {
		for _, objkey := range objkeys {
			key := objectStatusKey(objkey)
			item, err := txn.Get(key)
			if err != nil {
				return err
			}
			val, err := item.Value()
			if err != nil {
				return err
			}
			if ObjectStatus(val[0]) != ObjectStatus_LOCKED && ObjectStatus(val[0]) != ObjectStatus_ACTIVE {
				return ErrObjectCannotBeUnlocked
			}
			err = txn.Set(key, []byte{byte(ObjectStatus_ACTIVE)})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// AddTransaction create a new entry for the transaction seen by this node. This also create
// a new entry for the committed transaction set to false
func AddTransaction(store *badger.DB, txkey []byte, value []byte) error {
	return store.Update(func(txn *badger.Txn) error {
		seenkey := seenTxnKey(txkey)
		_, err := txn.Get(seenkey)
		if err == nil {
			return ErrTransactionAlreadyExists
		}
		err = txn.Set(seenkey, value)
		if err != nil {
			return err
		}
		commitkey := committedTxnKey(txkey)
		_, err = txn.Get(commitkey)
		if err == nil {
			return ErrTransactionAlreadyExists
		}
		err = txn.Set(commitkey, []byte{0})
		if err != nil {
			return err
		}
		return nil
	})
}

// GetTransaction return a transaction stored in database matching the given key.
// if the transaction key do not exists an error is returned
// returns the value of the transction, and if the transaction is committed or not as a boolean
func GetTransaction(store *badger.DB, txkey []byte) ([]byte, bool, error) {
	var txvalue []byte
	var txcommitted bool
	err := store.View(func(txn *badger.Txn) error {
		seenkey := seenTxnKey(txkey)
		item, err := txn.Get(seenkey)
		if err != nil {
			return nil
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		commitkey := committedTxnKey(txkey)
		item, err = txn.Get(commitkey)
		if err != nil {
			return err
		}
		status, err := item.Value()
		if err != nil {
			return err
		}
		txvalue = make([]byte, len(val))
		copy(txvalue, val)
		txcommitted = status[0] == 1
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return txvalue, txcommitted, nil
}

// GetObjectsFromStore return the list of objects corresponding to the list of keys
// order the same. if any of the keys do not match in database an error is returned
func GetObjects(store *badger.DB, keys [][]byte) ([]*Object, error) {
	objects := make([]*Object, 0, len(keys))
	err := store.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			o := &Object{
				Key: key,
			}
			objkey := objectKey(key)
			item, err := txn.Get(objkey)
			if err != nil {
				return err
			}
			val, err := item.Value()
			if err != nil {
				return err
			}
			o.Value = make([]byte, len(val))
			copy(o.Value, val)
			statuskey := objectStatusKey(key)
			item, err = txn.Get(statuskey)
			rawStatus, err := item.Value()
			if err != nil {
				return err
			}
			o.Status = ObjectStatus(rawStatus[0])
			objects = append(objects, o)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return objects, nil
}

// DeactivateObjects set to inactive all objects in the list
// this will return an error if one+ objects are already inactive
func DeactivateObjects(store *badger.DB, keys [][]byte) error {
	return store.Update(func(tx *badger.Txn) error {
		return setObjectsInactive(tx, keys)
	})
}

// CreateObjects
func CreateObjects(store *badger.DB, objs []*Object) error {
	return store.Update(func(tx *badger.Txn) error {
		return createObjects(tx, objs)
	})
}

// testing purpose only, allow us to create an new object in the node without consensus
// in a completely arbitrary way
func CreateObject(store *badger.DB, key, value []byte) (*Object, error) {
	var o *Object
	return o, store.Update(func(txn *badger.Txn) error {
		objkey := objectKey(key)
		_, err := txn.Get(objkey)
		if err == nil {
			return ErrObjectAlreadyExists
		}
		err = txn.Set(objkey, value)
		if err != nil {
			return err
		}
		objstatuskey := objectStatusKey(key)
		_, err = txn.Get(objstatuskey)
		if err == nil {
			return ErrObjectAlreadyExists
		}
		err = txn.Set(objstatuskey, []byte{byte(ObjectStatus_ACTIVE)})
		if err != nil {
			return err
		}
		o = &Object{
			Key:    key,
			Value:  value,
			Status: ObjectStatus_ACTIVE,
		}
		return nil
	})
}

// testing purpose only, allow us to delete an object in the node without consensus
// in a completely arbitrary way
func DeleteObjects(store *badger.DB, objkeys [][]byte) error {
	return store.Update(func(tx *badger.Txn) error {
		return setObjectsInactive(tx, objkeys)
	})
}

func FinishTransaction(store *badger.DB, txnkey []byte) error {
	return store.Update(func(txn *badger.Txn) error {
		finishedTxn := finishedTxnKey(txnkey)
		if err := txn.Set(finishedTxn, []byte{}); err != nil {
			return err
		}
		return nil
	})

}
func TxnFinished(store *badger.DB, txnkey []byte) (bool, error) {
	var ok bool
	var err error = store.View(func(txn *badger.Txn) error {
		key := finishedTxnKey(txnkey)
		_, err := txn.Get(key)
		if err != nil && err == badger.ErrKeyNotFound {
			ok = false
			return nil
		}
		if err != nil {
			return err
		}
		ok = true
		return nil
	})
	return ok, err
}
