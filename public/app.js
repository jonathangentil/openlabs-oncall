const API_URL = '/api/plantoes';
const PESSOAS_URL = '/api/pessoas';

const SISTEMAS_MAP = {
    "AAA": "AAA / FULFILLMENT / IAM / NADM",
    "ALTAIA": "ALTAIA / AM",
    "NETQ": "NETQ / SIGO",
    "NETWIN": "NETWIN"
};

// Pega o token salvo no login
const AUTH_TOKEN = localStorage.getItem('adminToken');

// Helper para cabeçalhos de autenticação
function getHeaders() {
    return {
        'Content-Type': 'application/json',
        'Authorization': AUTH_TOKEN || ''
    };
}

document.addEventListener('DOMContentLoaded', () => {
    carregarPlantoes();
    carregarPessoas();
    
    // Máscara de Telefone
    const inputTelefone = document.getElementById('novoContato');
    if (inputTelefone) {
        inputTelefone.addEventListener('input', function (e) {
            let v = e.target.value.replace(/\D/g, "");
            v = v.replace(/^(\d{2})(\d)/g, "($1) $2");
            v = v.replace(/(\d)(\d{4})$/, "$1-$2");
            e.target.value = v;
        });
    }
});

// --- PLANTÕES ---

async function carregarPlantoes() {
    try {
        const response = await fetch(API_URL);
        const plantoes = await response.json();
        
        const isAdmin = document.getElementById('listaAdmin');
        const tabelaBody = isAdmin ? document.getElementById('listaAdmin') : document.getElementById('listaCliente');
        const filtro = document.getElementById('filtroSistema') ? document.getElementById('filtroSistema').value : '';

        if (!tabelaBody) return;
        tabelaBody.innerHTML = '';

        const hoje = new Date();
        hoje.setHours(0,0,0,0);

        plantoes.forEach(p => {
            const nomeSistemaDisplay = SISTEMAS_MAP[p.sistema] || p.sistema;
            if(filtro && p.sistema !== filtro && nomeSistemaDisplay !== filtro) return;

            let isPassado = false;
            if (p.dataFim) {
                const partes = p.dataFim.split('-');
                const dataFimPlantao = new Date(partes[0], partes[1]-1, partes[2]);
                if (dataFimPlantao < hoje) {
                    isPassado = true;
                    if (!isAdmin) return; 
                }
            }

            const tr = document.createElement('tr');
            if (isAdmin) {
                const styleOpacidade = isPassado ? 'opacity: 0.5; background-color: #f9f9f9;' : '';
                tr.innerHTML = `
                    <td style="${styleOpacidade}"><span class="tag-system">${nomeSistemaDisplay}</span></td>
                    <td style="${styleOpacidade}">${p.periodo}</td>
                    <td style="${styleOpacidade}"><strong>${p.nome}</strong></td>
                    <td style="${styleOpacidade}">${p.contato}</td>
                    <td style="text-align:center; ${styleOpacidade}">
                        <button class="btn-icon delete" title="Remover" onclick="deletarPlantao(${p.id})">✕</button>
                    </td>
                `;
            } else {
                tr.innerHTML = `
                    <td><span class="tag-system">${nomeSistemaDisplay}</span></td>
                    <td>${p.periodo}</td>
                    <td><strong>${p.nome}</strong></td>
                    <td><a href="tel:${limparTelefone(p.contato)}" class="contact-link">📞 ${p.contato}</a></td>
                `;
            }
            tabelaBody.appendChild(tr);
        });

        if (!isAdmin && tabelaBody.children.length === 0) {
            tabelaBody.innerHTML = '<tr><td colspan="4" style="text-align:center; padding:2rem; color:#999;">Nenhum plantão vigente encontrado.</td></tr>';
        }

    } catch (error) { console.error("Erro ao carregar:", error); }
}

async function adicionarPlantao() {
    const sistema = document.getElementById('sistema').value;
    const dataInicio = document.getElementById('dataInicio').value;
    const dataFim = document.getElementById('dataFim').value; 
    const selectPessoa = document.getElementById('selectPessoa');
    
    if (!sistema || !dataInicio || !dataFim || selectPessoa.value === "") {
        return alert("Preencha todos os campos!");
    }

    const [nome, contato] = selectPessoa.value.split('|');
    const periodo = `De ${formatarDataSimples(dataInicio)} a ${formatarDataSimples(dataFim)}`;

    try {
        const response = await fetch(API_URL, {
            method: 'POST',
            headers: getHeaders(), 
            body: JSON.stringify({ sistema, periodo, nome, contato, dataFim })
        });

        if (!response.ok) {
            if (response.status === 401) {
                alert("Sessão expirada. Faça login novamente.");
                window.location.href = 'login.html';
                return;
            }
            const erroTexto = await response.text();
            alert("Erro: " + erroTexto);
            return;
        }

        document.getElementById('dataInicio').value = '';
        document.getElementById('dataFim').value = '';
        carregarPlantoes();
        alert("Plantão adicionado!");

    } catch (error) { alert("Erro de conexão."); }
}

async function deletarPlantao(id) {
    if (confirm('Remover este item?')) {
        const response = await fetch(`${API_URL}/${id}`, { 
            method: 'DELETE',
            headers: getHeaders()
        });
        
        if (response.status === 401) {
             window.location.href = 'login.html';
             return;
        }
        carregarPlantoes();
    }
}

// --- PESSOAS ---

async function carregarPessoas() {
    const listaEl = document.getElementById('listaPessoas');
    const selectEl = document.getElementById('selectPessoa');
    if (!listaEl) return; 

    // O script inline do admin.html cuida da exibição visual da lista de edição
    // Aqui garantimos que o select seja populado mesmo se a função inline falhar
    // mas na prática, a função sobrescrita no admin.html tem prioridade.
}

async function salvarPessoa() {
    const id = document.getElementById('pessoaId').value;
    const nome = document.getElementById('novoNome').value;
    const contato = document.getElementById('novoContato').value;

    if(!nome || !contato) return alert("Preencha nome e contato");

    const method = id ? 'PUT' : 'POST';
    const url = id ? `${PESSOAS_URL}/${id}` : PESSOAS_URL;

    const res = await fetch(url, {
        method: method,
        headers: getHeaders(),
        body: JSON.stringify({ nome, contato })
    });

    if (res.status === 401) {
        window.location.href = 'login.html';
        return;
    }

    cancelarEdicao(); 
    if (typeof window.carregarPessoas === 'function') window.carregarPessoas(); 
}

async function deletarPessoa(id) {
    if(confirm("Tem certeza?")) {
        const res = await fetch(`${PESSOAS_URL}/${id}`, { 
            method: 'DELETE',
            headers: getHeaders()
        });
        
        if (res.status === 401) {
            window.location.href = 'login.html';
            return;
        }

        if (typeof window.carregarPessoas === 'function') window.carregarPessoas();
    }
}

// --- UTIL ---
function prepararEdicao(id, nome, contato) {
    document.getElementById('pessoaId').value = id;
    document.getElementById('novoNome').value = nome;
    document.getElementById('novoContato').value = contato;
    const event = new Event('input');
    document.getElementById('novoContato').dispatchEvent(event);
    const btn = document.getElementById('btnSalvarPessoa');
    btn.innerText = "Salvar Alteração";
    document.getElementById('btnCancelarEdicao').style.display = "block";
}
function cancelarEdicao() {
    document.getElementById('pessoaId').value = '';
    document.getElementById('novoNome').value = '';
    document.getElementById('novoContato').value = '';
    document.getElementById('btnSalvarPessoa').innerText = "+ Adicionar";
    document.getElementById('btnCancelarEdicao').style.display = "none";
}
function limparTelefone(tel) { return tel ? tel.replace(/\D/g, '') : ''; }
function formatarDataSimples(dataIso) { if(!dataIso) return ""; const p = dataIso.split('-'); return `${p[2]}/${p[1]}`; }