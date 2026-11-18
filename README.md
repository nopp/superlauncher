# 🚀 Super Launcher

Super Launcher leve, elegante e auto-hospedável feito em **Go (Golang)** com **SQLite** integrado (sem dependência externa de CGO).

## ✨ Funcionalidades

1. **Barra de Pesquisa do Google**:
   - Centralizada no topo da tela inicial.
   - Digite sua pesquisa e aperte `Enter` para buscar diretamente no Google.
   - Relógio e data em tempo real com tipografia limpa.

2. **Grade de Aplicativos (Dashboard)**:
   - Exibe os atalhos para suas aplicações locais/web (ex: *Solid Voucher 3D*, *Workshop*, *Plex*, *Home Assistant*, etc.).
   - Ícones personalizados via upload (PNG, SVG, JPG, WebP, etc.) ou URL externa.
   - Fallback automático: se não houver ícone, gera uma tag estilizada com a inicial do app e cor única.
   - Opção para abrir em nova aba ou na mesma aba.

3. **Painel Administrativo Completo (`/admin`)**:
   - Protegido por senha (sessão segura via Cookie HMAC-SHA256).
   - **Adicionar (Create)** novo app com nome, link, descrição, ícone e ordem.
   - **Listar / Visualizar (Read)** com suporte à reordenação.
   - **Editar (Update)** qualquer aplicativo existente.
   - **Excluir (Delete)** com confirmação e limpeza automática de imagens antigas do disco.

---

## 🛠️ Como Executar

### 1. Compilar e Rodar Localmente

```bash
# Compilar o executável
go build -o superlauncher .

# Executar
./superlauncher
```

Por padrão, a aplicação roda na porta `:9090`.

Acesse no navegador:
- **Página Inicial (Launcher)**: [http://localhost:9090](http://localhost:9090)
- **Área Administrativa**: [http://localhost:9090/admin](http://localhost:9090/admin)

---

## ⚙️ Variáveis de Ambiente

Você pode configurar a aplicação definindo variáveis de ambiente antes de iniciar:

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `ADDR` | `:9090` | Endereço e porta em que o servidor irá escutar (ex: `:3000`, `0.0.0.0:9090`) |
| `DATA_DIR` | `data` | Pasta onde serão salvos o banco `superlauncher.db` e a pasta `uploads/` |
| `ADMIN_PASSWORD` | `admin` | Senha para acessar o painel `/admin` |

### Exemplo customizado:
```bash
ADMIN_PASSWORD="minhasenhasecreta" ADDR=":3000" ./superlauncher
```

---

## 📁 Estrutura de Arquivos

```text
superlauncher/
├── data/                  # Banco SQLite e uploads de ícones gerados em tempo de execução
├── static/                # CSS e Favicon embutidos no binário via go:embed
│   ├── favicon.svg
│   └── style.css
├── templates/             # Templates HTML (go:embed)
│   ├── form.html          # Criação e Edição de apps
│   ├── index.html         # Launcher com busca Google e grade de apps
│   ├── list.html          # Listagem do Admin
│   ├── login.html         # Login do Admin
│   └── partials.html      # Componentes reutilizáveis
├── handlers.go            # Rotas HTTP, autenticação de sessão e uploads
├── main.go                # Inicialização do servidor e configurações
├── store.go               # Camada de banco de dados SQLite (CRUD)
└── store_test.go          # Testes unitários do repositório
```
