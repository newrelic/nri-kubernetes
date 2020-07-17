package storage

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// setNow forces a different "current time" for the storage.
func setNow(newNow func() time.Time) {
	now = newNow
}

func TestDiskStorage_Struct(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	nowTime := time.Now()
	setNow(func() time.Time {
		return nowTime
	})

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// And a stored struct value
	type testStruct struct {
		FloatVal  float64
		StringVal string
		MapVal    map[string]interface{}
		StructVal struct {
			A float64
			B string
		}
	}
	stored := testStruct{
		1, "2",
		map[string]interface{}{"hello": "how are you", "fine": "and you?"},
		struct {
			A float64
			B string
		}{11, "22"},
	}
	assert.Nil(t, ds.Write("my-storage-test", stored))

	var read testStruct
	// When reading it from the disk
	ts, err := ds.Read("my-storage-test", &read)

	assert.Equal(t, stored, read)

	// As well as the insertion timestamp
	assert.Equal(t, nowTime.Unix(), ts)
	assert.Nil(t, err)
}

func TestDiskStorage_Map(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	nowTime := time.Now()
	setNow(func() time.Time {
		return nowTime
	})

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// And a stored map
	stored := map[string]interface{}{
		"1": "2",
		"3": map[string]interface{}{"hello": "how are you", "fine": "and you?"},
		"4": 5.0,
	}
	assert.Nil(t, ds.Write("my-storage-test", stored))

	// When reading it from the disk
	var read map[string]interface{}
	ts, err := ds.Read("my-storage-test", &read)

	// An map equal to the original has been returned
	assert.Equal(t, stored, read)

	// As well as the insertion timestamp
	assert.Equal(t, nowTime.Unix(), ts)
	assert.Nil(t, err)
}

func TestDiskStorage_Array(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	nowTime := time.Now()
	setNow(func() time.Time {
		return nowTime
	})

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// And a stored array
	stored := []interface{}{"1", 2.0, "3", map[string]interface{}{"hello": "how are you", "fine": "and you?"}}
	assert.Nil(t, ds.Write("my-storage-test", stored))

	// When reading it from the disk
	var read []interface{}
	ts, err := ds.Read("my-storage-test", &read)

	// It returns an array equal to the original
	assert.Equal(t, stored, read)

	// As well as the insertion timestamp
	assert.Equal(t, nowTime.Unix(), ts)
	assert.Nil(t, err)
}

func TestDiskStorage_String(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	nowTime := time.Now()
	setNow(func() time.Time {
		return nowTime
	})

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// And a stored string
	stored := "hello my good friend"
	assert.Nil(t, ds.Write("my-storage-test", stored))

	// When reading it from the disk
	var read string
	ts, err := ds.Read("my-storage-test", &read)

	// It returns a string equal to the original
	assert.Equal(t, stored, read)

	// As well as the insertion timestamp
	assert.Equal(t, nowTime.Unix(), ts)
	assert.Nil(t, err)
}

func TestDiskStorage_Number(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	nowTime := time.Now()
	setNow(func() time.Time {
		return nowTime
	})

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// And a stored integer
	stored := int(123456)
	assert.Nil(t, ds.Write("my-storage-test", stored))

	// When reading it from the disk
	var read int
	ts, err := ds.Read("my-storage-test", &read)

	// It returns the copy of the original number
	assert.Equal(t, stored, read)

	// As well as the insertion timestamp
	assert.Equal(t, nowTime.Unix(), ts)
	assert.Nil(t, err)
}

func TestDiskStorage_Overwrite(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// And a stored record
	assert.Nil(t, ds.Write("my-storage-test", "initial Value"))

	// When this record is overwritten
	assert.Nil(t, ds.Write("my-storage-test", "overwritten value"))

	// The read operation returns the last version of the record
	var read string
	_, err = ds.Read("my-storage-test", &read)

	assert.Equal(t, "overwritten value", read)
	assert.Nil(t, err)
}

func TestDiskStorage_NotFound(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// When trying to access an nonexistent record
	var read string
	_, err = ds.Read("my-storage-test", &read)

	// The storage returns an error
	assert.NotNil(t, err)
}

func TestJSONDiskStorage_Delete(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// And a stored record
	assert.Nil(t, ds.Write("my-storage-test", "initial Value"))

	// When removing the stored record
	assert.Nil(t, ds.Delete("my-storage-test"))

	// When trying to access an nonexistent record
	var read string
	_, err = ds.Read("my-storage-test", &read)

	// The storage returns an error
	assert.NotNil(t, err)
}

func TestJSONDiskStorage_DeleteUnexistent(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "disk_storage")
	assert.Nil(t, err)

	// Given a JSONDiskStorage
	var ds Storage = NewJSONDiskStorage(rootDir)

	// When trying to remove a non-existing record
	err = ds.Delete("my-storage-test")

	// The storage does not return any error
	assert.Nil(t, err)
}
