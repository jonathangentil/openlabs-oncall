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

	"github.com/joho/godotenv" // Import para ler .env
	_ "github.com/lib/pq"
	"golang.org/x/crypto/acme/autocert"
)

//go:embed public
var publicFS embed.FS

// Valores padrão (fallback) caso não estejam no .env
const (
	defaultAdminPassword = "admin_123"
	defaultDomainName    = "plantao.openlabs.com.br"
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

	// 1. Tenta carregar variáveis do arquivo .env
	// Se der erro (ex: em produção não tem arquivo), apenas avisa e segue usando env vars do sistema
	if err := godotenv.Load(); err != nil {
		log.Println("Info: Arquivo .env não encontrado. Usando variáveis de ambiente do sistema.")
	}

	// Configuração de diretórios para certificados
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
	// Pega do .env ou usa defaults
	dbHost := getEnv("DB_HOST", "localhost")
	// Nota: DB_PORT_EXTERNAL é o nome que usamos no .env para a porta exposta
	dbPort := getEnv("DB_PORT_EXTERNAL", "5432")
	dbUser := getEnv("DB_USER", "admin")
	dbPass := getEnv("DB_PASS", "admin_123") // Fallback de segurança fraca apenas para dev
	dbName := getEnv("DB_NAME", "escala_db")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	fmt.Printf("Conectando ao Postgres em %s:%s...\n", dbHost, dbPort)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Erro ao abrir driver do banco:", err)
	}

	// Testar conexão (Ping)
	err = db.Ping()
	if err != nil {
		log.Fatalf("ERRO FATAL: Não foi possível conectar ao banco de dados em %s:%s.\nVerifique se o Docker está rodando e se as credenciais no .env estão corretas.\nErro: %v", dbHost, dbPort, err)
	}
	defer db.Close()
	fmt.Println(">> Conexão com Banco de Dados estabelecida com sucesso!")

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

	domain := getEnv("DOMAIN_NAME", defaultDomainName)

	if *devMode {
		fmt.Println("------------------------------------------------")
		fmt.Println(">> MODO DEV ATIVADO")
		fmt.Println(">> Servidor rodando em: http://localhost:8080")
		fmt.Println("------------------------------------------------")
		log.Fatal(http.ListenAndServe(":8080", nil))
	} else {
		fmt.Printf(">> MODO PRODUÇÃO: HTTPS para %s na porta 443...\n", domain)
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domain),
			Cache:      autocert.DirCache(certDir),
		}
		server := &http.Server{
			Addr: ":443",
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
				NextProtos:     []string{"h2", "http/1.1", "acme-tls/1"},
			},
		}

		// Redirecionamento HTTP -> HTTPS
		go http.ListenAndServe(":80", certManager.HTTPHandler(nil))

		err = server.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatal("Erro ao iniciar servidor HTTPS:", err)
		}
	}
}

// Helper para ler env vars com valor padrão
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Middleware de Autenticação
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			next(w, r)
			return
		}
		token := r.Header.Get("Authorization")

		// Compara com a senha do .env ou o default
		adminPass := getEnv("ADMIN_PASSWORD", defaultAdminPassword)

		if token != adminPass {
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

	adminPass := getEnv("ADMIN_PASSWORD", defaultAdminPassword)

	if req.Password == adminPass {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Retorna o próprio token (senha) para o front armazenar
		w.Write([]byte(`{"token": "` + adminPass + `"}`))
	} else {
		http.Error(w, "Senha incorreta", http.StatusUnauthorized)
	}
}

func createTables() {
	// Postgres: SERIAL para auto-incremento
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
		log.Println("Erro ao criar tabela plantoes:", err)
	}

	queryPessoas := `CREATE TABLE IF NOT EXISTS pessoas (
		id SERIAL PRIMARY KEY,
		nome TEXT, 
		contato TEXT
	);`

	_, err = db.Exec(queryPessoas)
	if err != nil {
		log.Println("Erro ao criar tabela pessoas:", err)
	}
}

func handlePlantoes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, sistema, periodo, nome, contato, data_fim FROM plantoes ORDER BY data_fim ASC")
		if err != nil {
			log.Println("Erro Select:", err)
			http.Error(w, "Erro no Banco de Dados", 500)
			return
		}
		defer rows.Close()

		var lista []Plantao
		for rows.Next() {
			var p Plantao
			var df sql.NullString // Trata NULLs se houver
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
			http.Error(w, "JSON inválido", 400)
			return
		}

		// Postgres: Placeholders $1, $2... e RETURNING id
		var newID int
		err := db.QueryRow(`
			INSERT INTO plantoes(sistema, periodo, nome, contato, data_fim) 
			VALUES($1, $2, $3, $4, $5) RETURNING id`,
			p.Sistema, p.Periodo, p.Nome, p.Contato, p.DataFim,
		).Scan(&newID)

		if err != nil {
			log.Println("Erro Insert Plantão:", err)
			http.Error(w, "Erro ao inserir no Banco", 500)
			return
		}

		p.ID = newID
		json.NewEncoder(w).Encode(p)
	}
}

func handlePlantaoDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/plantoes/")

		// Placeholder $1
		_, err := db.Exec("DELETE FROM plantoes WHERE id = $1", idStr)
		if err != nil {
			log.Println("Erro Delete Plantão:", err)
			http.Error(w, "Erro ao deletar no Banco", 500)
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
			http.Error(w, "Erro no Banco de Dados", 500)
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
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}

		// RETURNING id e placeholders
		var newID int
		err := db.QueryRow("INSERT INTO pessoas(nome, contato) VALUES($1, $2) RETURNING id", p.Nome, p.Contato).Scan(&newID)

		if err != nil {
			log.Println("Erro Insert Pessoa:", err)
			http.Error(w, "Erro ao inserir pessoa", 500)
			return
		}
		p.ID = newID
		json.NewEncoder(w).Encode(p)
	}
}

func handlePessoaOperacoes(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/pessoas/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", 400)
		return
	}

	if r.Method == "DELETE" {
		_, err := db.Exec("DELETE FROM pessoas WHERE id = $1", id)
		if err != nil {
			log.Println("Erro Delete Pessoa:", err)
			http.Error(w, "Erro ao deletar", 500)
			return
		}
		w.WriteHeader(http.StatusOK)

	} else if r.Method == "PUT" {
		var p Pessoa
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}

		// Placeholders $1, $2, $3
		_, err := db.Exec("UPDATE pessoas SET nome = $1, contato = $2 WHERE id = $3", p.Nome, p.Contato, id)
		if err != nil {
			log.Println("Erro Update Pessoa:", err)
			http.Error(w, "Erro ao atualizar", 500)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
