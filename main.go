package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"database/sql"
	"flag"
	_ "modernc.org/sqlite"
	_"github.com/mattn/go-sqlite3"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tealeg/xlsx"
)

type User struct {
	FirstName string
	Faculty  string
	Kurs	string
	Gruppe	string
	ID        int
	Admin	int
}

var flags int 

func main() {
	outputFileName := flag.String("output", "mibs.sqlite", "output file name")
	flag.Parse()

	// Создайте подключение к базе данных
	db, err := sql.Open("sqlite3", *outputFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// Создать таблицу для данных MIB, если она не существует
	err = createTable(db)
    if err != nil {
        log.Fatal(err)
    }
	// иницилизируем нового бота
	bot, err := tgbotapi.NewBotAPI("5650995329:AAGbDMN2_eNOmLIPbqqDq6_DnGhOEBk9deE")
	if err != nil {
		log.Fatal(err)
	}

	//слушаем обновления
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	// загружаем список пользователей из таблицы
	users, err := loadUserData("tabl.xlsx")
	if err != nil {
		log.Fatal(err)
	}
	existingUsers, err := readUsersFromDB(db)
	if err != nil {
		log.Fatal(err)
	}

	for id, user := range existingUsers {
		users[id] = user
	}

	// слушаем обновления
	for update := range updates {
		if update.Message == nil {
			continue
		}
		userID := update.Message.From.ID
		if strings.Contains(strings.ToLower(update.Message.Text), "/admin") {
			admin(bot, update, users, db)
		} else {
			switch strings.ToLower(update.Message.Text) {
			case "/start":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprint("Welcome ", userID, "!"))
				bot.Send(msg)
			case "/help":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Available commands:\n/start - Start the bot\n/help - Show this help message")
				bot.Send(msg)
			case "/register":
				flags = 1
				register(bot, update, update.Message.Chat.ID, updates, users, db)
				for flags != 0 {
					time.Sleep(time.Second * 1)
				}
			case "gfhjkmflvbyf":
				password(bot, update, userID, db, users)
			default:
				user, ok := users[userID]
				if !ok || user.Admin != 1 {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You don't have permission to use this command.")
					bot.Send(msg)
				} else {
					message := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/admin "))
					rows, err := db.Query("SELECT id FROM users")
					if err != nil {
						log.Println(err)
						return
					}
					defer rows.Close()

					// отправка сообщений зарегестрированным пользователям
					for rows.Next() {
						var userID int
						if err := rows.Scan(&userID); err != nil {
							log.Println(err)
							continue
						}

						msg := tgbotapi.NewMessage(int64(userID), message)
						if _, err := bot.Send(msg); err != nil {
							log.Println(err)
						}
					}

					// сообщение подверждения для админа
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Message sent to all registered users.")
					bot.Send(msg)
				}
			}
		}
		// проверка регистрации пользователя
		_, ok := users[userID]
		if !ok {
			// если пользователь не зарегестрирван
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You are not registered. Would you like to register now? (Please enter /register)")
			bot.Send(msg)
		}
	}
}

func loadUserData(filename string) (map[int]User, error) {
	xlFile, err := xlsx.OpenFile(filename)
	if err != nil {
		return nil, err
	}

	users := make(map[int]User)
	for _, row := range xlFile.Sheets[0].Rows[1:] {
		id, err := strconv.Atoi(row.Cells[4].String())
		if err != nil {
			id = rand.Int()
		}
		user := User{
			FirstName:	row.Cells[0].String(),
			Faculty:	row.Cells[1].String(),
			Kurs:		row.Cells[2].String(),
			Gruppe:		row.Cells[3].String(),
			ID:			id,
		}
		users[user.ID] = user
	}
	fmt.Println("Готова к работе:")
	return users, nil
}

func register(bot *tgbotapi.BotAPI, update tgbotapi.Update, id int64, updates tgbotapi.UpdatesChannel, users map[int]User, db *sql.DB ) {
	// Отправляем пользователю приветственное сообщение
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать! Чтобы зарегистрироваться, введите свои данные в формате \"ФИО факультет курс группа\".\n Example: Путятина Анастасия Андреевна ФГиИБ 3 ИСИТ")
	bot.Send(msg)
	// Обрабатываем сообщения пользователя
	for {
		select {
		case update := <-updates:
			// Проверяем, что сообщение отправлено пользователем, который вызвал /register
			fmt.Println(id, " ", int(update.Message.Chat.ID))
			if update.Message == nil || id != update.Message.Chat.ID {
				if id != update.Message.Chat.ID {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Подождите, сейчас кто-то регистрируется, попробуйте позже")
					bot.Send(msg)
				}
				continue
			}

			// Проверяем, что сообщение пользователя содержит имя и фамилию
			parts := strings.Split(update.Message.Text, " ")
			if len(parts) != 6 {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неверный формат. Введите свои данные в формате \"ФИО факультет курс группа\".")
				bot.Send(msg)
				continue
			}

			// Отправляем сообщение о успешной регистрации
			firstName := parts[0] + " " + parts[1] + " " + parts[2]
			i:=filterUsersByName(users, firstName)
			if i == 0 {
				flags = 0
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вас нет в списке группы, если это ошибка напишите старосте!")
				bot.Send(msg)
				return
			} else {
				delete(users, i)
			}
			user := User{
				FirstName: firstName,
				Faculty:   parts[3],
				Kurs:      parts[4],
				Gruppe:    parts[5],
				ID:        int(id),
			}
			users[user.ID] = user
			flags = 0
			stmt, err := db.Prepare("INSERT INTO Users (FirstName, Faculty, Kurs, Gruppe, ID, Admin) VALUES (?, ?, ?, ?, ?, ?)")
			if err != nil {
				log.Fatal(err)
			}
			defer stmt.Close()
			_, err = stmt.Exec(firstName, parts[3], parts[4], parts[5], user.ID, 0)
			if err != nil {
				log.Fatal(err)
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы успешно зарегистрированы!")
			bot.Send(msg)
			return

		case <-time.After(time.Second * 60):
			// Если прошло более 60 секунд, завершаем функцию
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Время регистрации истекло.")
			bot.Send(msg)
			flags = 0
			return
		}
	}
}

func filterUsersByName(m map[int]User, substr string) int {
	for id, user := range m {
		if strings.Contains(user.FirstName, substr) {
			return id
		}
	}
	return 0
}

func createTable(db *sql.DB) error {
    _, err := db.Exec("CREATE TABLE IF NOT EXISTS Users (FirstName TEXT, Faculty TEXT, Kurs TEXT, Gruppe TEXT, ID INTEGER, Admin INTEGER)")
    if err != nil {
        return err
    }
    return nil
}

func admin(bot *tgbotapi.BotAPI, update tgbotapi.Update, users map[int]User, db *sql.DB) {
	userID := update.Message.From.ID
	user, ok := users[userID]
	if !ok || user.Admin != 1 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You don't have permission to use this command.")
		bot.Send(msg)
		return
	}

	message := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/admin"))
	if message == "" {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please enter a message to send to registered users.")
		bot.Send(msg)
		return
	}
	arr := strings.Split(message, " ")
	var rows *sql.Rows
	var err error
	if arr[0] == "-" && arr[1] == "-" && arr[2] == "-" && len(arr) > 3{
		rows, err = db.Query("SELECT id FROM users")
		if err != nil {
			log.Println(err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid argumens example: /admin - - - .(сообщение).")
			bot.Send(msg)
			return
		}
		message = strings.Join(arr[3:], " ")
	} else if arr[0] != "-" && arr[1] == "-" && arr[2] == "-" && len(arr) > 3{
		rows, err = db.Query("SELECT id FROM Users WHERE Faculty = ?", arr[0])
		if err != nil {
			log.Println(err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid argumens example: /admin ФГиИБ - - .(сообщение).")
			bot.Send(msg)
			return
		}
		message = strings.Join(arr[3:], " ")
	} else if arr[0] != "-" && arr[1] != "-" && arr[2] == "-" && len(arr) > 3{
		rows, err = db.Query("SELECT id FROM Users WHERE Faculty = ? AND Kurs = ?", arr[0], arr[1])
		if err != nil {
			log.Println(err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid argumens example: /admin ФГиИБ 4 - .(сообщение).")
			bot.Send(msg)
			return
		}
		message = strings.Join(arr[2:], " ")
	} else if arr[0] != "-" && arr[1] != "-" && arr[2] != "-" && len(arr) > 3{
		rows, err = db.Query("SELECT id FROM Users WHERE Faculty = ? AND Kurs = ? AND Gruppe = ?", arr[0], arr[1], arr[2])
		if err != nil {
			log.Println(err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid argumens example: /admin ФГиИБ 4 ПИ .(сообщение).")
			bot.Send(msg)
			return
		}
		message = strings.Join(arr[3:], " ")
	} else if arr[0] == "-" && arr[1] != "-" && arr[2] != "-" && len(arr) > 3{
		rows, err = db.Query("SELECT id FROM Users WHERE Kurs = ? AND Gruppe = ?", arr[1], arr[2])
		if err != nil {
			log.Println(err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid argumens example: /admin ФГиИБ 4 ПИ .(сообщение).")
			bot.Send(msg)
		return
		}
	} else if arr[0] == "-" && arr[1] != "-" && arr[2] == "-" && len(arr) > 3{
		rows, err = db.Query("SELECT id FROM Users WHERE Kurs = ?", arr[1])
		if err != nil {
			log.Println(err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid argumens example: /admin ФГиИБ 4 ПИ .(сообщение).")
			bot.Send(msg)
			return
		}
		message = strings.Join(arr[3:], " ")
	} else if arr[0] == "-" && arr[1] == "-" && arr[2] != "-" && len(arr) > 3{
		rows, err = db.Query("SELECT id FROM Users WHERE Gruppe = ?", arr[2])
		if err != nil {
			log.Println(err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid argumens example: /admin ФГиИБ 4 ПИ .(сообщение).")
			bot.Send(msg)
			return
		}
		message = strings.Join(arr[3:], " ")
	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid arguments, example: /admin ФГиИБ 4 ПИ .(сообщение).")
		bot.Send(msg)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			log.Println(err)
			continue
		}
		fmt.Println(userID)
		msg := tgbotapi.NewMessage(int64(userID), message)
		if _, err := bot.Send(msg); err != nil {
			log.Println(err)
		}
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Message sent to all registered users.")
	bot.Send(msg)
	fmt.Println("otpravleno")
}

func password(bot *tgbotapi.BotAPI, update tgbotapi.Update, userID int, db *sql.DB, users map[int]User) {
	_, err := db.Exec("UPDATE users SET admin = ? WHERE id = ?", 1, userID)
    if err != nil {
        log.Fatal(err)
    }
	user, ok := users[userID]
	if !ok {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You are not registered. Would you like to register now? (Please enter /register)")
		bot.Send(msg)
		return
	}
	user.Admin = 1
	users[userID] = user
    msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("User %d is now an admin.", userID))
    bot.Send(msg)
}

func readUsersFromDB(db *sql.DB) (map[int]User, error) {
	users := make(map[int]User)
	rows, err := db.Query("SELECT FirstName, Faculty, Kurs, Gruppe, ID, Admin FROM Users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		err := rows.Scan(&user.FirstName, &user.Faculty, &user.Kurs, &user.Gruppe, &user.ID, &user.Admin)
		if err != nil {
			return nil, err
		}
		users[user.ID] = user
	}

	return users, nil
}
