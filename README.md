# Open Labs On-Call (Escala de Plantonistas)

Sistema web leve para gerenciamento e visualização de escalas de plantão de suporte técnico. Desenvolvido para ser simples, performático e fácil de implantar em infraestrutura Linux corporativa.

![Status](https://img.shields.io/badge/status-active-success.svg)
![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=flat&logo=go&logoColor=white)
![SQLite](https://img.shields.io/badge/sqlite-%2307405e.svg?style=flat&logo=sqlite&logoColor=white)

## 📋 Funcionalidades

- **Visão do Cliente (Pública):** Consulta rápida de quem está de plantão no momento, filtrado por sistema/área.
- **Visão Admin (Protegida):** Painel para cadastro, edição e remoção de plantonistas e escalas.
- **Autenticação:** Proteção de rotas de escrita via Token/Senha simples.
- **Single Binary:** O frontend (HTML/CSS/JS) é embutido dentro do executável Go, facilitando o deploy.
- **Banco de Dados:** SQLite local (arquivo `.db`), sem necessidade de servidores adicionais.

## 🚀 Tecnologias

- **Backend:** Go (Golang) 1.21+
- **Database:** SQLite (Driver Pure Go `modernc.org/sqlite`)
- **Frontend:** HTML5, CSS3, JavaScript (Vanilla)
- **Deploy:** Systemd (Linux Service)

## 💻 Rodando Localmente (Desenvolvimento)

1. Clone o repositório:

    ```bash
    git clone https://github.com/seu-usuario/openlabs-oncall.git
    cd openlabs-oncall
    ```

2. Instale as dependências:

    ```bash
    go mod tidy
    ```

3. Execute o projeto:

    ```bash
    go run .
    ```

4. Acesse no navegador:

    - **Cliente:** `http://localhost:8080`
    - **Admin:** `http://localhost:8080/admin.html`
    - **Senha Padrão:** `admin123` (Configurável no `main.go`)

## 🛠️ Compilação para Produção (Linux)

Para rodar em servidores Linux (inclusive versões antigas como Oracle Linux ou CentOS), gere um binário estático:

### No Windows PowerShell:

```powershell
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o escala-plantao
```

## ☁️ Instalação no Servidor (Deploy)

### 1. Preparar Diretório

```bash
sudo mkdir -p /opt/plantao
sudo chmod +x /opt/plantao/escala-plantao
```

### 2. Criar Serviço Systemd

```ini
[Unit]
Description=Servico Escala Plantonistas Open Labs
After=network.target

[Service]
WorkingDirectory=/opt/plantao
ExecStart=/opt/plantao/escala-plantao
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
```

### 3. Iniciar o Serviço

```bash
sudo systemctl daemon-reload
sudo systemctl enable plantao
sudo systemctl start plantao
```

### 4. Verificar Status e Logs

```bash
sudo systemctl status plantao
sudo journalctl -u plantao -f
```

## 🔒 Segurança e Acesso Externo

### Exemplo Nginx

```nginx
server {
    listen 80;
    server_name plantao.openlabs.com.br;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## ❓ Troubleshooting

### Erro: GLIBC_2.34 not found
Recompile usando `CGO_ENABLED=0`.

### Erro: 203/EXEC no Systemd
Verifique permissão (`chmod +x`) e arquitetura correta.

## ✔️ Licença
Projeto livre para uso interno e corporativo.
