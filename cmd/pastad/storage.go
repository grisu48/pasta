package main

import (
	"database/sql"
	"math/rand"
)

var db *sql.DB

type Bin struct {
	Id           string
	Owner        string
	CreationDate int64
	ExpireDate   int64
	Size         int64
}

func DbInitialize(db *sql.DB) error {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `bins` (`id` VARCHAR(64) PRIMARY KEY, `owner` VARCHAR(64), `createdate` int, `expiredate` int, `size` int);")
	if err != nil {
		panic(err)
	}
	return nil
}

func FetchBin(id string) (Bin, error) {
	bin := Bin{Id: ""}
	stmt, err := db.Prepare("SELECT `id`,`owner`,`createdate`,`expiredate`,`size` FROM `bins` WHERE `id` = ? LIMIT 1;")
	if err != nil {
		return bin, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(id)
	if err != nil {
		return bin, err
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&bin.Id, &bin.Owner, &bin.CreationDate, &bin.ExpireDate, &bin.Size)
	}
	return bin, nil
}

func InsertBin(bin Bin) error {
	stmt, err := db.Prepare("INSERT INTO `bins`(`id`,`owner`,`createdate`,`expiredate`,`size`) VALUES (?,?,?,?,?);")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(bin.Id, bin.Owner, bin.CreationDate, bin.ExpireDate, bin.Size)
	return err
}

func DeleteBin(id string) error {
	stmt, err := db.Prepare("DELETE FROM `bins` WHERE `id` = ?;")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(id)
	return err
}

func RandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func GenerateRandomBinId(n int) (string, error) {
	for {
		id := RandomString(n)
		bin, err := FetchBin(id)
		if err != nil {
			return "", err
		}
		if bin.Id == "" {
			return id, nil
		}
	}
}
