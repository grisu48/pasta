package main

import (
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"
)

var testBowl PastaBowl

func TestMain(m *testing.M) {
	// Initialisation
	rand.Seed(time.Now().UnixNano())
	testBowl.Directory = "pasta_test"
	os.Mkdir(testBowl.Directory, os.ModePerm)
	defer os.RemoveAll(testBowl.Directory)
	// Run tests
	ret := m.Run()
	os.Exit(ret)
}

func TestMetadata(t *testing.T) {
	var err error
	var pasta, p1, p2, p3 Pasta

	p1.AttachmentFilename = "file1"
	p1.Mime = "text/plain"
	if err = testBowl.InsertPasta(&p1); err != nil {
		t.Fatalf("Error inserting pasta 1: %s", err)
		return
	}
	if p1.Id == "" {
		t.Fatal("Pasta 1 id not set")
		return
	}
	if p1.Token == "" {
		t.Fatal("Pasta 1 id not set")
		return
	}
	p2.AttachmentFilename = "file2"
	p2.Mime = "application/json"
	if err = testBowl.InsertPasta(&p2); err != nil {
		t.Fatalf("Error inserting pasta 2: %s", err)
		return
	}
	// Insert pasta with given ID and Token
	p3Id := testBowl.GenerateRandomBinId(12)
	p3Token := RandomString(20)
	p3.Id = p3Id
	p3.Token = p3Token
	p3.AttachmentFilename = "file3"
	p3.Mime = "text/rtf"
	if err = testBowl.InsertPasta(&p3); err != nil {
		t.Fatalf("Error inserting pasta 3: %s", err)
		return
	}
	if p3.Id != p3Id {
		t.Fatal("Pasta 3 id mismatch")
		return
	}
	if p3.Token != p3Token {
		t.Fatal("Pasta 3 id mismatch")
		return
	}

	pasta, err = testBowl.GetPasta(p1.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 1: %s", err)
		return
	}
	if pasta != p1 {
		t.Fatal("Pasta 1 mismatch")
		return
	}
	pasta, err = testBowl.GetPasta(p2.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 2: %s", err)
		return
	}
	if pasta != p2 {
		t.Fatal("Pasta 2 mismatch")
		return
	}
	pasta, err = testBowl.GetPasta(p3.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 3: %s", err)
		return
	}
	if pasta != p3 {
		t.Fatal("Pasta 3 mismatch")
		return
	}

	if err = testBowl.DeletePasta(p1.Id); err != nil {
		t.Fatalf("Error deleting pasta 1: %s", err)
	}
	pasta, err = testBowl.GetPasta(p1.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 1 (after delete): %s", err)
		return
	}
	if pasta.Id != "" {
		t.Fatal("Pasta 1 exists after delete")
		return
	}
	// Ensure pasta 2 and 3 are not affected if we delete pasta 1
	pasta, err = testBowl.GetPasta(p2.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 2 after deleting pasta 1: %s", err)
		return
	}
	if pasta != p2 {
		t.Fatal("Pasta 2 mismatch after deleting pasta 1")
		return
	}
	pasta, err = testBowl.GetPasta(p3.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 3 after deleting pasta 1: %s", err)
		return
	}
	if pasta != p3 {
		t.Fatal("Pasta 3 mismatch after deleteing pasta 1")
		return
	}
	// Delete also pasta 2
	if err = testBowl.DeletePasta(p2.Id); err != nil {
		t.Fatalf("Error deleting pasta 2: %s", err)
	}
	pasta, err = testBowl.GetPasta(p2.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 2 (after delete): %s", err)
		return
	}
	if pasta.Id != "" {
		t.Fatal("Pasta 2 exists after delete")
		return
	}
	pasta, err = testBowl.GetPasta(p3.Id)
	if err != nil {
		t.Fatalf("Error getting pasta 3 after deleting pasta 2: %s", err)
		return
	}
	if pasta != p3 {
		t.Fatal("Pasta 3 mismatch after deleting pasta 2")
		return
	}
}

func TestBlobs(t *testing.T) {
	var err error
	var p1, p2 Pasta

	// Contents
	testString1 := RandomString(4096 * 8)
	testString2 := RandomString(4096 * 8)

	if err = testBowl.InsertPasta(&p1); err != nil {
		t.Fatalf("Error inserting pasta 1: %s", err)
		return
	}
	file, err := testBowl.GetPastaWriter(p1.Id)
	if err != nil {
		t.Fatalf("Error getting pasta file 1: %s", err)
		return
	}
	defer file.Close()
	if _, err = file.Write([]byte(testString1)); err != nil {
		t.Fatalf("Error writing to pasta file 1: %s", err)
		return
	}
	if err = file.Close(); err != nil {
		t.Fatalf("Error closing pasta file 1: %s", err)
		return
	}
	if err = testBowl.InsertPasta(&p2); err != nil {
		t.Fatalf("Error inserting pasta 2: %s", err)
		return
	}
	file, err = testBowl.GetPastaWriter(p2.Id)
	if err != nil {
		t.Fatalf("Error getting pasta file 2: %s", err)
		return
	}
	defer file.Close()
	if _, err = file.Write([]byte(testString2)); err != nil {
		t.Fatalf("Error writing to pasta file 2: %s", err)
		return
	}
	if err = file.Close(); err != nil {
		t.Fatalf("Error closing pasta file 2: %s", err)
		return
	}
	// Fetch contents now
	file, err = testBowl.GetPastaReader(p1.Id)
	if err != nil {
		t.Fatalf("Error getting pasta reader 1: %s", err)
		return
	}
	buf, err := ioutil.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Error reading pasta 1: %s", err)
		return
	}
	if testString1 != string(buf) {
		t.Fatal("Mismatch: pasta 1 contents")
		t.Logf("Bytes: Read %d, Expected %d", len(buf), len(([]byte(testString1))))
		return
	}
	// Same for pasta 2
	file, err = testBowl.GetPastaReader(p2.Id)
	if err != nil {
		t.Fatalf("Error getting pasta reader 2: %s", err)
		return
	}
	buf, err = ioutil.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Error reading pasta 2: %s", err)
		return
	}
	if testString2 != string(buf) {
		t.Fatal("Mismatch: pasta 2 contents")
		t.Logf("Bytes: Read %d, Expected %d", len(buf), len(([]byte(testString2))))
		return
	}

	// Check if pasta 1 can be deleted and the contents of pasta 2 are still OK afterwards
	if err = testBowl.DeletePasta(p1.Id); err != nil {
		t.Fatalf("Error deleting pasta 1: %s", err)
	}
	file, err = testBowl.GetPastaReader(p2.Id)
	if err != nil {
		t.Fatalf("Error getting pasta reader 2: %s", err)
		return
	}
	buf, err = ioutil.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Error reading pasta 2: %s", err)
		return
	}
	if testString2 != string(buf) {
		t.Fatal("Mismatch: pasta 2 contents")
		t.Logf("Bytes: Read %d, Expected %d", len(buf), len(([]byte(testString2))))
		return
	}

}
