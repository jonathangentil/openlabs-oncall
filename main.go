package main

import (
	"crypto/tls"
	"database/sql"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/acme/autocert"
)

//go:embed public
var publicFS embed.FS

const (
	adminPassword = "J727HCfmF4dL9P36n9rr"
	domainName    = "plantao.openlabs.com.br"
)

type Plantao struct {
	ID      int    `json:"id"`
	Sistema string `json:"sistema"`
	Periodo string `json:"periodo"`
	Nome    string `json:"nome"`
	Contato string `json:"contato"`
	DataFim string `json:"dataFim"`
}

type Pessoa struct {
	ID      int    `json:"id"`
	Nome    string `json:"nome"`
	Contato string `json:"contato"`
}

type LoginRequest struct {
	Password string `json:"password"`
}

var db *sql.DB

func main() {
	var err error

	// Configuração de diretórios (Mantido para certificados)
	ex, err := os.Executable()
	if err != nil {
		log.Fatal("Erro ao descobrir diretório do executável:", err)
	}
	exePath := filepath.Dir(ex)
	certDir := filepath.Join(exePath, "certs")

	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		os.Mkdir(certDir, 0755)
	}

	// --- CONEXÃO COM POSTGRES ---
	// Se houver variáveis de ambiente, usa elas. Senão, usa o padrão do Docker local.
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "15432")
	dbUser := getEnv("DB_USER", "admin")
	dbPass := getEnv("DB_PASS", "rr01dYZA6ltjP11lu0e2")
	dbName := getEnv("DB_NAME", "escala_db")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	fmt.Println("Conectando ao Postgres...")
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Erro ao abrir conexão com o banco:", err)
	}

	// Testar conexão
	err = db.Ping()
	if err != nil {
		log.Fatal("Não foi possível conectar ao banco de dados (verifique se o Docker está rodando):", err)
	}
	defer db.Close()

	createTables()

	// Configuração de arquivos estáticos
	publicDir, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatal("Erro ao carregar arquivos estáticos:", err)
	}
	http.Handle("/", http.FileServer(http.FS(publicDir)))

	// Rotas da API
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/plantoes", authMiddleware(handlePlantoes))
	http.HandleFunc("/api/plantoes/", authMiddleware(handlePlantaoDelete))
	http.HandleFunc("/api/pessoas", authMiddleware(handlePessoas))
	http.HandleFunc("/api/pessoas/", authMiddleware(handlePessoaOperacoes))

	// --- INICIALIZAÇÃO DO SERVIDOR ---
	devMode := flag.Bool("dev", false, "Rodar em modo local (localhost) na porta 8080")
	flag.Parse()

	if *devMode {
		fmt.Println("------------------------------------------------")
		fmt.Println(">> MODO DEV ATIVADO")
		fmt.Println(">> Servidor rodando em: http://localhost:8080")
		fmt.Println("------------------------------------------------")
		log.Fatal(http.ListenAndServe(":8080", nil))
	} else {
		fmt.Printf(">> MODO PRODUÇÃO: HTTPS para %s na porta 443...\n", domainName)
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domainName),
			Cache:      autocert.DirCache(certDir),
		}
		server := &http.Server{
			Addr: ":443",
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
				NextProtos:     []string{"h2", "http/1.1", "acme-tls/1"},
			},
		}
		go http.ListenAndServe(":80", certManager.HTTPHandler(nil))
		err = server.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatal("Erro ao iniciar servidor HTTPS:", err)
		}
	}
}

// Helper para ler env vars
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			next(w, r)
			return
		}
		token := r.Header.Get("Authorization")
		if token != adminPassword {
			http.Error(w, "Acesso Negado.", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Método inválido", 405)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	if req.Password == adminPassword {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token": "` + adminPassword + `"}`))
	} else {
		http.Error(w, "Senha incorreta", http.StatusUnauthorized)
	}
}

func createTables() {

	queryPlantoes := `CREATE TABLE IF NOT EXISTS plantoes (
		id SERIAL PRIMARY KEY,
		sistema TEXT, 
		periodo TEXT, 
		nome TEXT, 
		contato TEXT, 
		data_fim TEXT
	);`

	_, err := db.Exec(queryPlantoes)
	if err != nil {
		log.Println("Erro tabela plantoes:", err)
	}

	queryPessoas := `CREATE TABLE IF NOT EXISTS pessoas (
		id SERIAL PRIMARY KEY,
		nome TEXT, 
		contato TEXT
	);`

	_, err = db.Exec(queryPessoas)
	if err != nil {
		log.Println("Erro tabela pessoas:", err)
	}
}

func handlePlantoes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, sistema, periodo, nome, contato, data_fim FROM plantoes ORDER BY data_fim ASC")
		if err != nil {
			log.Println("Erro Select:", err)
			http.Error(w, "Erro BD", 500)
			return
		}
		defer rows.Close()
		var lista []Plantao
		for rows.Next() {
			var p Plantao
			var df sql.NullString
			rows.Scan(&p.ID, &p.Sistema, &p.Periodo, &p.Nome, &p.Contato, &df)
			p.DataFim = df.String
			lista = append(lista, p)
		}
		if lista == nil {
			lista = []Plantao{}
		}
		json.NewEncoder(w).Encode(lista)
	} else if r.Method == "POST" {
		var p Plantao
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "JSON", 400)
			return
		}

		var newID int
		err := db.QueryRow(`
			INSERT INTO plantoes(sistema, periodo, nome, contato, data_fim) 
			VALUES($1, $2, $3, $4, $5) RETURNING id`,
			p.Sistema, p.Periodo, p.Nome, p.Contato, p.DataFim,
		).Scan(&newID)

		if err != nil {
			log.Println("Erro Insert Plantão:", err)
			http.Error(w, "Erro BD", 500)
			return
		}

		p.ID = newID
		json.NewEncoder(w).Encode(p)
	}
}

func handlePlantaoDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/plantoes/")

		_, err := db.Exec("DELETE FROM plantoes WHERE id = $1", idStr)
		if err != nil {
			log.Println("Erro Delete Plantão:", err)
			http.Error(w, "Erro BD", 500)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func handlePessoas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, nome, contato FROM pessoas ORDER BY nome ASC")
		if err != nil {
			log.Println("Erro Select Pessoas:", err)
			http.Error(w, "Erro BD", 500)
			return
		}
		defer rows.Close()
		var lista []Pessoa
		for rows.Next() {
			var p Pessoa
			rows.Scan(&p.ID, &p.Nome, &p.Contato)
			lista = append(lista, p)
		}
		if lista == nil {
			lista = []Pessoa{}
		}
		json.NewEncoder(w).Encode(lista)
	} else if r.Method == "POST" {
		var p Pessoa
		json.NewDecoder(r.Body).Decode(&p)

		var newID int
		err := db.QueryRow("INSERT INTO pessoas(nome, contato) VALUES($1, $2) RETURNING id", p.Nome, p.Contato).Scan(&newID)

		if err != nil {
			log.Println("Erro Insert Pessoa:", err)
			http.Error(w, "Erro BD", 500)
			return
		}
		p.ID = newID
		json.NewEncoder(w).Encode(p)
	}
}

func handlePessoaOperacoes(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/pessoas/")
	id, _ := strconv.Atoi(idStr)

	if r.Method == "DELETE" {

		_, err := db.Exec("DELETE FROM pessoas WHERE id = $1", id)
		if err != nil {
			log.Println("Erro Delete Pessoa:", err)
		}
		w.WriteHeader(http.StatusOK)
	} else if r.Method == "PUT" {
		var p Pessoa
		json.NewDecoder(r.Body).Decode(&p)

		_, err := db.Exec("UPDATE pessoas SET nome = $1, contato = $2 WHERE id = $3", p.Nome, p.Contato, id)
		if err != nil {
			log.Println("Erro Update Pessoa:", err)
		}
		w.WriteHeader(http.StatusOK)
	}
}
