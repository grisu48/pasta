package main

import (
	"database/sql"
	"math/rand"
	"os"
	"testing"
	"time"
)

const testFilename = "test.db"

func TestMain(m *testing.M) {
	var err error

	// Initialisation
	rand.Seed(time.Now().UnixNano())
	os.Remove(testFilename)
	db, err = sql.Open("sqlite3", testFilename)
	if err != nil {
		panic(err)
	}
	if err = DbInitialize(db); err != nil {
		panic(err)
	}
	defer db.Close()
	// Run tests
	ret := m.Run()
	os.Exit(ret)
}

func TestBins(t *testing.T) {
	var err error
	var bin Bin
	bin1 := Bin{Id: "a", Owner: "user1", CreationDate: 1, ExpireDate: 1000, Size: 10}
	bin2 := Bin{Id: "b", Owner: "user2", CreationDate: 2, ExpireDate: 2000, Size: 50}

	if err = InsertBin(bin1); err != nil {
		t.Fatalf("Error inserting bin 1: %s", err)
		return
	}
	if err = InsertBin(bin2); err != nil {
		t.Fatalf("Error inserting bin 2: %s", err)
		return
	}
	if bin, err = FetchBin(bin1.Id); err != nil {
		t.Fatalf("Error getting bin 1: %s", err)
		return
	} else if bin != bin1 {
		t.Fatal("Bin 1 mismatch", err)
		return
	}
	if bin, err = FetchBin(bin2.Id); err != nil {
		t.Fatalf("Error getting bin 2: %s", err)
		return
	} else if bin != bin2 {
		t.Fatal("Bin 2 mismatch", err)
		return
	}
	if err = DeleteBin(bin1.Id); err != nil {
		t.Fatalf("Error deleting bin 1: %s", err)
		return
	}
	if bin, err = FetchBin(bin1.Id); err != nil {
		t.Fatalf("Error getting bin 1 after delete: %s", err)
		return
	} else if bin.Id != "" {
		t.Fatal("Bin 1 still exists after delete", err)
		return
	}
	// Check if bin2 is still existing
	if bin, err = FetchBin(bin2.Id); err != nil {
		t.Fatalf("Error getting bin 2: %s", err)
		return
	} else if bin != bin2 {
		t.Fatal("Bin 2 mismatch after deleting", err)
		return
	}
	// Now delete bin2 as well
	if err = DeleteBin(bin2.Id); err != nil {
		t.Fatalf("Error deleting bin 1: %s", err)
		return
	}
	if bin, err = FetchBin(bin2.Id); err != nil {
		t.Fatalf("Error getting bin 2 after delete: %s", err)
		return
	} else if bin.Id != "" {
		t.Fatal("Bin 2 still exists after delete", err)
		return
	}
}
