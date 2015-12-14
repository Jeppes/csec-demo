package main

import (
	"bytes"
	"fmt"
	"github.com/gorilla/sessions"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	session *mgo.Session
}

var (
	s     Server
	store = sessions.NewCookieStore([]byte("secret")) // Pls use a real secret
)

func main() {
	session, err := mgo.Dial("localhost")
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	s.session = session
	defer s.session.Close()

	http.HandleFunc("/transfer", transfer)
	http.HandleFunc("/login", login)
	http.HandleFunc("/account", account)
	http.Handle("/", http.FileServer(http.Dir(".")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type User struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	Name    string        `json:"name", bson:"name,omitempty"`
	Balance int           `bson:"balance,omitempty"`
}

type Transaction struct {
	Message  string        `bson:"message",omitempty`
	Amount   int           `bson:"amount"`
	From     bson.ObjectId `bson:"from"`
	FromUser string        `bson:"from_user"`
	To       bson.ObjectId `bson:"to"`
	ToUser   string        `bson:"to_user"`
	Date     time.Time     `bson:"time"`
}

type Account struct {
	User         User
	Transactions template.HTML
}

func transfer(w http.ResponseWriter, r *http.Request) {
	userSession, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}

	name := userSession.Values["user"]
	session := s.session.Copy()
	defer session.Close()

	amount, _ := strconv.Atoi(r.FormValue("amount"))
	receiver_name := r.FormValue("receiver")
	message := r.FormValue("message")

	var receiver User
	var user User
	users := session.DB("csec-demo").C("users")
	err = users.Find(User{Name: receiver_name}).One(&receiver)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	err = users.Find(User{Name: name.(string)}).One(&user)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	transactions := session.DB("csec-demo").C("transactions")
	transaction := Transaction{Amount: amount, To: receiver.ID, ToUser: receiver.Name, From: user.ID, FromUser: user.Name, Date: time.Now(), Message: message}
	transactions.Insert(transaction)

	users.UpdateId(receiver.ID, bson.M{"$set": bson.M{"balance": receiver.Balance + amount}})
	users.UpdateId(user.ID, bson.M{"$set": bson.M{"balance": user.Balance - amount}})

	log.Println("Transaction completed from:", user.Name, "to:", receiver.Name)

	http.Redirect(w, r, "/account", 302)
}

func account(w http.ResponseWriter, r *http.Request) {
	userSession, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	name := userSession.Values["user"].(string)
	if name == "" {
		http.Error(w, err.Error(), 401)
		return
	}

	session := s.session.Copy()
	defer session.Close()

	var user User
	users := session.DB("csec-demo").C("users")
	err = users.Find(User{Name: name}).One(&user)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var transactions []Transaction
	cTransactions := session.DB("csec-demo").C("transactions")
	err = cTransactions.Find(bson.M{"$or": []bson.M{bson.M{"from": user.ID}, bson.M{"to": user.ID}}}).Limit(5).Sort("-time").All(&transactions)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var message string
	var buffer bytes.Buffer
	for _, transaction := range transactions {
		if transaction.From == user.ID {
			message = fmt.Sprintf("to %s", transaction.ToUser)
		} else {
			message = fmt.Sprintf("from %s", transaction.FromUser)
		}
		buffer.WriteString(fmt.Sprintf(
			`<li>
				<span class="date"><i>%s %d, %d</i></span>
				<br>	
				<strong>$%d </strong>%s
				<br>
				<i>Message: </i>%s
			</li>`, transaction.Date.Month(), transaction.Date.Day(), transaction.Date.Year(), transaction.Amount, message, transaction.Message))
	}

	account := Account{User: user, Transactions: template.HTML(buffer.String())}

	tmpl, err := template.New("account.html").ParseFiles("account.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	tmpl.Execute(w, account)

	log.Println("Account page generate for user:", user.Name)
}

func login(w http.ResponseWriter, r *http.Request) {
	var user User

	name := r.FormValue("name")

	session := s.session.Copy()
	defer session.Close()

	users := session.DB("csec-demo").C("users")

	err := users.Find(User{Name: name}).One(&user)
	if err != nil {
		user = User{ID: bson.NewObjectId(), Name: name, Balance: 1000}
		err = users.Insert(user)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		log.Println("User", user.Name, "created with id:", user.ID)
	}

	userSession, err := store.Get(r, "session-name")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	userSession.Values["user"] = user.Name
	userSession.Save(r, w)

	log.Println("User:", user.Name, "logged in!")

	http.Redirect(w, r, "/account", 302)
}
