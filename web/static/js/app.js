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
