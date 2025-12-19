package main

import (
	"crypto/tls"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/acme/autocert"
	_ "modernc.org/sqlite"
)

//go:embed public
var publicFS embed.FS

const (
	adminPassword = "admin123"
	domainName    = "plantao.openlabs.com.br" // Domínio configurado
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

	// Configuração de diretórios
	ex, err := os.Executable()
	if err != nil {
		log.Fatal("Erro ao descobrir diretório do executável:", err)
	}
	exePath := filepath.Dir(ex)
	dbPath := filepath.Join(exePath, "escala.db")
	certDir := filepath.Join(exePath, "certs") // Pasta para salvar os certificados

	// Garante que a pasta de certificados existe
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		os.Mkdir(certDir, 0755)
	}

	fmt.Println("Banco de Dados:", dbPath)
	fmt.Println("Certificados serão salvos em:", certDir)

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal("Erro fatal ao abrir banco:", err)
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

	// --- CONFIGURAÇÃO SSL (LETS ENCRYPT) ---
	fmt.Printf("Iniciando servidor HTTPS para %s na porta 443...\n", domainName)

	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domainName),
		Cache:      autocert.DirCache(certDir),
	}

	server := &http.Server{
		Addr: ":443",
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
			// NextProtos é essencial para o desafio TLS-ALPN-01 (Porta 443 apenas)
			NextProtos: []string{"h2", "http/1.1", "acme-tls/1"},
		},
	}

	// Inicia o servidor com TLS.
	// As strings vazias fazem ele usar o config do GetCertificate acima.
	err = server.ListenAndServeTLS("", "")
	if err != nil {
		log.Fatal("Erro ao iniciar servidor HTTPS:", err)
	}
}

// --- FUNÇÕES AUXILIARES (Inalteradas) ---

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
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS plantoes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sistema TEXT, periodo TEXT, nome TEXT, contato TEXT, data_fim TEXT
	);`)
	if err != nil {
		log.Println("Erro tabela plantoes:", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS pessoas (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT, contato TEXT
	);`)
	if err != nil {
		log.Println("Erro tabela pessoas:", err)
	}
}

func handlePlantoes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, sistema, periodo, nome, contato, data_fim FROM plantoes ORDER BY data_fim ASC")
		if err != nil {
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
		res, err := db.Exec("INSERT INTO plantoes(sistema, periodo, nome, contato, data_fim) VALUES(?, ?, ?, ?, ?)", p.Sistema, p.Periodo, p.Nome, p.Contato, p.DataFim)
		if err != nil {
			http.Error(w, "Erro BD", 500)
			return
		}
		id, _ := res.LastInsertId()
		p.ID = int(id)
		json.NewEncoder(w).Encode(p)
	}
}

func handlePlantaoDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/plantoes/")
		_, err := db.Exec("DELETE FROM plantoes WHERE id = ?", idStr)
		if err != nil {
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
		res, err := db.Exec("INSERT INTO pessoas(nome, contato) VALUES(?, ?)", p.Nome, p.Contato)
		if err != nil {
			http.Error(w, "Erro BD", 500)
			return
		}
		id, _ := res.LastInsertId()
		p.ID = int(id)
		json.NewEncoder(w).Encode(p)
	}
}

func handlePessoaOperacoes(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/pessoas/")
	id, _ := strconv.Atoi(idStr)
	if r.Method == "DELETE" {
		db.Exec("DELETE FROM pessoas WHERE id = ?", id)
		w.WriteHeader(http.StatusOK)
	} else if r.Method == "PUT" {
		var p Pessoa
		json.NewDecoder(r.Body).Decode(&p)
		db.Exec("UPDATE pessoas SET nome = ?, contato = ? WHERE id = ?", p.Nome, p.Contato, id)
		w.WriteHeader(http.StatusOK)
	}
}
