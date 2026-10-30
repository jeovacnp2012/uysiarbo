// Comportamentos de interface do UYSI ARBO. Carregado antes do Alpine: registra tudo em "alpine:init".
document.addEventListener('alpine:init', () => {
    // Tema claro/escuro, guardado no navegador (o atributo inicial é aplicado no <head> para não piscar).
    Alpine.store('tema', {
        escuro: document.documentElement.dataset.tema === 'escuro',
        alternar() {
            this.escuro = !this.escuro;
            if (this.escuro) {
                document.documentElement.dataset.tema = 'escuro';
            } else {
                delete document.documentElement.dataset.tema;
            }
            try {
                localStorage.setItem('uysi-tema', this.escuro ? 'escuro' : 'claro');
            } catch (e) {}
        },
    });

    // Menu lateral recolhível no computador (no celular ele abre como gaveta). A escolha fica no navegador.
    Alpine.store('menu', {
        alternar() {
            const raiz = document.documentElement;
            const recolher = raiz.dataset.menu !== 'recolhido';
            if (recolher) {
                raiz.dataset.menu = 'recolhido';
            } else {
                delete raiz.dataset.menu;
            }
            try {
                localStorage.setItem('uysi-menu', recolher ? 'recolhido' : '');
            } catch (e) {}
        },
    });

    // Mensagens rápidas. Disparo: window.dispatchEvent(new CustomEvent('toast', {detail: {kind, message}}))
    // ou, vindo do servidor, o cabeçalho HX-Trigger: {"toast": {"kind": "sucesso", "message": "..."}}.
    Alpine.data('toasts', () => ({
        itens: [],
        proximo: 1,
        adicionar(detalhe) {
            const id = this.proximo++;
            this.itens.push({ id, kind: detalhe.kind || 'info', message: detalhe.message || '' });
            setTimeout(() => this.remover(id), 5000);
        },
        remover(id) {
            this.itens = this.itens.filter((t) => t.id !== id);
        },
    }));

    // Editor de permissões de um papel: busca por texto e marcar/desmarcar em bloco.
    Alpine.data('editorPermissoes', () => ({
        busca: '',
        mostra(el) {
            const q = this.busca.trim().toLowerCase();
            return q === '' || el.dataset.busca.includes(q);
        },
        temResultado(el) {
            const q = this.busca.trim().toLowerCase();
            return q === '' || [...el.querySelectorAll('.perm')].some((p) => p.dataset.busca.includes(q));
        },
        marcar(raiz, valor) {
            raiz.querySelectorAll('input[name=perm]:not(:disabled)').forEach((i) => {
                i.checked = valor;
            });
        },
    }));

    // Campo de senha com revelar/ocultar (exigido em todo campo de senha do sistema).
    Alpine.data('campoSenha', () => ({
        visivel: false,
        alternar() {
            this.visivel = !this.visivel;
        },
    }));
});

// Presença: cada janela tem um identificador próprio (a mesma pessoa pode ter várias abertas) e,
// ao fechar a tela, avisa o servidor para o aviso sumir logo para os outros.
window.uysiAba = (window.crypto && crypto.randomUUID ? crypto.randomUUID() : String(Date.now()) + Math.random()).replace(/-/g, '').replace(/\./g, '');
window.addEventListener('pagehide', () => {
    const csrf = document.body && document.body.getAttribute('hx-headers');
    let token = '';
    try {
        token = JSON.parse(csrf || '{}')['X-CSRF-Token'] || '';
    } catch (e) {}
    document.querySelectorAll('.presenca[data-recurso]').forEach((el) => {
        const dados = new FormData();
        dados.set('recurso', el.dataset.recurso);
        dados.set('id', el.dataset.id);
        dados.set('aba', window.uysiAba);
        dados.set('csrf', token);
        navigator.sendBeacon('/presenca/sair', dados);
    });
});

// Parâmetros: campo com data-so-se="campo=valor;outro=valor" só aparece quando todas as condições valem.
function uysiSoSe(raiz) {
    const els = [...(raiz || document).querySelectorAll('[data-so-se]')].filter((el) => el.dataset.soSe);
    if (!els.length) return;
    const aplicar = () => {
        els.forEach((el) => {
            const ok = el.dataset.soSe.split(';').every((c) => {
                const [nome, valor] = c.split('=');
                const f = document.querySelector('[name="' + nome + '"]');
                return f && f.value === valor;
            });
            el.style.display = ok ? '' : 'none';
        });
    };
    els.forEach((el) => el.dataset.soSe.split(';').forEach((c) => {
        const f = document.querySelector('[name="' + c.split('=')[0] + '"]');
        if (f && !f.dataset.soSeLigado) {
            f.dataset.soSeLigado = '1';
            f.addEventListener('change', aplicar);
        }
    }));
    aplicar();
}
document.addEventListener('DOMContentLoaded', () => uysiSoSe(document));
document.addEventListener('htmx:load', (e) => uysiSoSe(e.target));

// Abas de Parâmetros: o conteúdo da aba troca dentro da página (#aba-area). Resposta sem a área (outra tela) vira
// navegação normal; erros de validação (400, 409...) com a área também são mostrados dentro da aba.
document.addEventListener('htmx:beforeSwap', (e) => {
    const d = e.detail;
    if (!d.target || d.target.id !== 'aba-area') return;
    const txt = d.serverResponse || '';
    if (txt.indexOf('id="aba-area"') < 0) {
        d.shouldSwap = false;
        if (d.requestConfig && d.requestConfig.verb === 'get' && d.pathInfo) window.location.href = d.pathInfo.requestPath;
        return;
    }
    if (d.xhr.status >= 400) {
        d.shouldSwap = true;
        d.isError = false;
    }
});
