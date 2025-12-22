# Open Labs | Oncall

Sistema de gerenciamento e visualização de escalas de plantão técnico.  
A aplicação fornece uma interface pública para consulta de plantonistas vigentes e um painel administrativo para gestão da equipe e dos horários.

---

## Tecnologias

- **Backend:** Go (Golang) 1.25  
- **Banco de Dados:** PostgreSQL 15 (via Docker)  
- **Frontend:** HTML5, CSS3, Vanilla JS (Single Page Application embedded)  
- **Infraestrutura:** Docker Compose e Systemd (Oracle Linux)  
- **Segurança:** HTTPS automático (Let's Encrypt / Autocert)

---

## Funcionalidades

### Visualização Pública
- Consulta rápida de quem está de plantão no momento
- Filtro por sistema: AAA, ALTAIA, NETQ, NETWIN

### Painel Administrativo
- Autenticação via token
- Cadastro, edição e remoção de membros da equipe
- Montagem de escala por período
- Histórico de escalas

### Outros Recursos
- Integração Docker: banco de dados isolado em container
- Modo híbrido:
  - Execução local (`-dev`) em HTTP
  - Produção com HTTPS automático

---

## Pré-requisitos

- Go (versão 1.21 ou superior)
- Docker e Docker Compose
- Git

---

## Configuração e Execução (Local)

### 1. Clone o repositório

```bash
git clone https://jonathangentil/escala-plantao.git
cd escala-plantao
```

### 2. Suba o Banco de Dados

O projeto utiliza um `docker-compose.yml` que sobe o PostgreSQL na porta **15432**.

```bash
docker-compose up -d
```

### 3. Execute a Aplicação

Utilize a flag `-dev` para rodar em modo HTTP na porta **8080**.  
A aplicação irá conectar automaticamente no banco local.

```bash
go run . -dev
```

### 4. Acesse

- Interface pública: http://localhost:8080  
- Login Admin: http://localhost:8080/login.html  
- Senha Admin (padrão): `admin_123`  
  - Definida diretamente no `main.go`

---

## Deploy em Produção (Linux Server)

Em produção, a aplicação roda como um serviço **Systemd** e o banco de dados via **Docker**.

### 1. Banco de Dados

Certifique-se de que o Docker está rodando e inicie o container:

```bash
cd /opt/plantao
docker-compose up -d
```

### 2. Compilação

Gere o binário compatível com Linux AMD64:

```powershell
# No PowerShell
$env:CGO_ENABLED="0"
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o escala-plantao
```

### 3. Instalação do Serviço

- Mova o binário para `/opt/plantao`
- Configure o arquivo `/etc/systemd/system/plantao.service`

#### Variáveis de Ambiente Necessárias

O Systemd deve injetar as variáveis para conectar na porta externa do Docker (**15432**):

```ini
Environment="DB_HOST=localhost"
Environment="DB_PORT=5432"
Environment="DB_USER=USUARIO_ADMIN"
Environment="DB_PASS=SENHA_BANCO_DE_DADOS"
Environment="DB_NAME=NOME_BANCO_DE_DADOS"
```

### 4. Comandos de Gerenciamento

```bash
# Iniciar serviço
sudo systemctl start plantao

# Verificar logs
sudo journalctl -u plantao -f
```

---

## Estrutura do Projeto

```text
/
├── main.go               # Código fonte principal (Go Server)
├── docker-compose.yml    # Orquestração do PostgreSQL
├── go.mod / go.sum       # Gerenciamento de dependências
├── public/               # Frontend estático (embarcado no binário)
│   ├── index.html        # Visão pública
│   ├── admin.html        # Painel administrativo
│   ├── login.html        # Tela de login
│   ├── app.js            # Lógica do frontend
│   └── style.css         # Estilização
└── certs/                # Certificados SSL automáticos (gerados em produção)
```

---

## Variáveis de Ambiente

A aplicação aceita as seguintes variáveis de ambiente (padrões definidos no código):

| Variável   | Padrão        | Descrição                         |
|------------|---------------|-----------------------------------|
| DB_HOST    | localhost     | Host do PostgreSQL                |
| DB_PORT    | 15432         | Porta mapeada no Docker Host      |
| DB_USER    | admin         | Usuário do banco de dados         |
| DB_PASS    | adminpassword | Senha do banco de dados           |
| DB_NAME    | escala_db     | Nome do database                  |

---
