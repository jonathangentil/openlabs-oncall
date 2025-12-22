# Manual de Operação – Sistema de Escala de Plantonistas (v2.1)

**Aplicação:** Escala de Plantão – Open Labs  
**Tecnologia:** Go (Golang) + PostgreSQL (Docker)  
**Execução:** Aplicação via Systemd / Banco de Dados via Docker Compose  
**URL:** https://plantao.openlabs.com.br  

---

## 1. Arquitetura do Sistema

O sistema opera em uma **arquitetura híbrida**, garantindo performance, segurança e facilidade de gestão.

### Componentes

- **Backend**
  - Binário único escrito em **Go**
  - Serve API e arquivos estáticos
  - Gerenciado pelo **Systemd**

- **Banco de Dados**
  - **PostgreSQL 15**
  - Executando em container **Docker** isolado

- **Comunicação**
  - Conexão via TCP
  - Porta **15432** (mapeada para `localhost`)

- **Configuração**
  - **Local:** leitura de arquivo `.env`
  - **Produção:** variáveis de ambiente injetadas pelo Systemd

- **Segurança**
  - HTTPS automático via **Let’s Encrypt**
  - Porta **443**

---

## 2. Estrutura de Arquivos

Todos os componentes residem em `/opt/plantao`.

| Caminho | Descrição | Permissão / Dono |
|-------|----------|------------------|
| `/opt/plantao/` | Diretório raiz | `opc:opc` |
| `/opt/plantao/escala-plantao` | Executável principal (Go) | `opc:opc (+x)` |
| `/opt/plantao/docker-compose.yml` | Orquestração do banco | `opc:opc` |
| `/opt/plantao/.env` | Configuração local (opcional em prod) | `opc:opc` |
| `/opt/plantao/certs/` | Certificados HTTPS | `opc:opc` |
| `/etc/systemd/system/plantao.service` | Serviço do sistema | `root:root` |

---

## 3. Configuração do Serviço (Systemd)

Em produção, as credenciais do banco são **injetadas diretamente pelo serviço**.

**Arquivo:** `/etc/systemd/system/plantao.service`

```ini
[Unit]
Description=Servico Escala Plantao
After=network.target

[Service]
# Segurança: Roda com usuário comum
User=opc
Group=opc

# Diretório de trabalho
WorkingDirectory=/opt/plantao/

# Caminho absoluto do binário
ExecStart=/opt/plantao/escala-plantao

# Permite ao usuário 'opc' abrir portas baixas (443)
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Reinicia automaticamente se cair
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

## 4. Gerenciamento do Ambiente

### 4.1 Aplicação (Backend Go)

```bash
sudo systemctl status plantao
sudo journalctl -u plantao -f
sudo systemctl restart plantao
sudo systemctl stop plantao
```

### 4.2 Banco de Dados (Docker)

```bash
docker ps
cd /opt/plantao
docker-compose up -d
docker logs -f openlabs_postgres
```

---

## 5. Processo de Atualização (Deploy)

### 5.1 Compilação (Windows / Dev)

```powershell
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o escala-plantao
```

### 5.2 Atualização no Servidor (Oracle Linux)

```bash
sudo systemctl stop plantao
sudo chmod +x /opt/plantao/escala-plantao
sudo chown -R opc:opc /opt/plantao
sudo restorecon -Rv /opt/plantao/
sudo systemctl start plantao
```

---

## 6. Backup e Recuperação

### 6.1 Backup do Banco de Dados

```bash
docker exec openlabs_postgres pg_dump -U admin escala_db > /opt/plantao/backup_$(date +%F).sql
```

### 6.2 Backup dos Certificados

Copiar o diretório:

```
/opt/plantao/certs/
```
---
## 7. Variáveis de Ambiente

| Variável           | Descrição                                                        | Exemplo de Valor        | Onde é usada?       |
| ------------------ | ---------------------------------------------------------------- | ----------------------- | ------------------- |
| `DB_HOST`          | Endereço do servidor do banco de dados.                          | localhost               | Go (Backend)        |
| `DB_PORT_EXTERNAL` | Porta externa exposta pelo Docker para conexão.                  | 15432                   | Go + Docker Compose |
| `DB_USER`          | Usuário para autenticação no PostgreSQL.                         | admin                   | Go + Docker Compose |
| `DB_PASS`          | Senha do usuário do banco.                                       | admin123                | Go + Docker Compose |
| `DB_NAME`          | Nome do banco de dados a ser criado ou utilizado pela aplicação. | escala_db               | Go + Docker Compose |
| `ADMIN_PASSWORD`   | Senha de login no painel administrativo e token de API.          | admin123                | Go (Backend)        |
| `DOMAIN_NAME`      | Domínio utilizado para geração do certificado HTTPS.             | plantao.openlabs.com.br | Go (Backend)        |


---

## 8. Troubleshooting

### 8.1 Erro: code=exited, status=203/EXEC

```bash
sudo restorecon -Rv /opt/plantao/
```

### 8.2 Erro: connect: connection refused

- Verifique se o container está ativo (`docker ps`)
- Confirme o mapeamento da porta **15432**

### 8.3 Erro: password authentication failed

- Verifique a senha no `plantao.service`
- Confirme se é a mesma do `docker-compose.yml`
