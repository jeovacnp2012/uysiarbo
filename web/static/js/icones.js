// Pacote de ícones do sistema: traço fino de 24x24 (estilo Lucide, licença ISC), desenhados como SVG inline.
// Os botões ganham o ícone certo pelo texto ("Salvar", "Excluir", "Novo bairro"...), em toda tela, inclusive
// nas que chegam por HTMX. Para um botão sem ícone, use data-sem-icone; para escolher à mão, data-icone="nome".
(function () {
    'use strict';

    var ICONES = {
        plus: '<path d="M5 12h14"/><path d="M12 5v14"/>',
        save: '<path d="M15.2 3a2 2 0 0 1 1.4.6l3.8 3.8a2 2 0 0 1 .6 1.4V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"/><path d="M17 21v-7a1 1 0 0 0-1-1H8a1 1 0 0 0-1 1v7"/><path d="M7 3v4a1 1 0 0 0 1 1h7"/>',
        trash: '<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><path d="M10 11v6"/><path d="M14 11v6"/>',
        pencil: '<path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/>',
        back: '<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>',
        next: '<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>',
        x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
        check: '<path d="M20 6 9 17l-5-5"/>',
        search: '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
        filter: '<path d="M10 20a1 1 0 0 0 .553.895l2 1A1 1 0 0 0 14 21v-7a2 2 0 0 1 .517-1.341L21.74 4.67A1 1 0 0 0 21 3H3a1 1 0 0 0-.742 1.67l7.225 7.989A2 2 0 0 1 10 14z"/>',
        undo: '<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>',
        refresh: '<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>',
        pin: '<path d="M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0"/><circle cx="12" cy="10" r="3"/>',
        eye: '<path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/>',
        logout: '<path d="m16 17 5-5-5-5"/><path d="M21 12H9"/><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>',
        upload: '<path d="M12 3v12"/><path d="m17 8-5-5-5 5"/><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>',
        download: '<path d="M12 15V3"/><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m7 10 5 5 5-5"/>',
        pause: '<rect x="14" y="4" width="4" height="16" rx="1"/><rect x="6" y="4" width="4" height="16" rx="1"/>',
        key: '<path d="m15.5 7.5 2.3 2.3a1 1 0 0 0 1.4 0l2.1-2.1a1 1 0 0 0 0-1.4L19 4"/><path d="m21 2-9.6 9.6"/><circle cx="7.5" cy="15.5" r="5.5"/>',
        file: '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/>',
        list: '<path d="M3 5h.01"/><path d="M3 12h.01"/><path d="M3 19h.01"/><path d="M8 5h13"/><path d="M8 12h13"/><path d="M8 19h13"/>',
        send: '<path d="M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z"/><path d="m21.854 2.147-10.94 10.939"/>',
        home: '<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .709-1.528l7-5.999a2 2 0 0 1 2.582 0l7 5.999A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
        book: '<path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H19a1 1 0 0 1 1 1v18a1 1 0 0 1-1 1H6.5a1 1 0 0 1 0-5H20"/>',
        bell: '<path d="M10.268 21a2 2 0 0 0 3.464 0"/><path d="M3.262 15.326A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.673C19.41 13.956 18 12.499 18 8A6 6 0 0 0 6 8c0 4.499-1.411 5.956-2.738 7.326"/>',
        moon: '<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>',
        grupos: '<path d="M7.5 4.27 9 5.15"/><path d="M16.5 4.27 15 5.15"/><path d="M21 8.5v7a2 2 0 0 1-1 1.73l-6 3.46a2 2 0 0 1-2 0l-6-3.46A2 2 0 0 1 4 15.5v-7a2 2 0 0 1 1-1.73l6-3.46a2 2 0 0 1 2 0l6 3.46A2 2 0 0 1 21 8.5z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/>',
        sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/>'
    };

    // Primeira regra que casar com o texto do botão (em minúsculas, sem acento) escolhe o ícone.
    var REGRAS = [
        [/^(salvar|gravar)/, 'save'],
        [/^(aplicar|marcar|confirmar)/, 'check'],
        [/^publicar/, 'send'],
        [/^(novo|nova|adicionar|criar|registrar|incluir)/, 'plus'],
        [/^(excluir|descartar|retirar|remover|apagar)/, 'trash'],
        [/^editar/, 'pencil'],
        [/^voltar ao inicio/, 'home'],
        [/^voltar/, 'back'],
        [/^cancelar/, 'x'],
        [/^(filtrar|pesquisar|buscar)/, 'filter'],
        [/^(limpar|reativar|restaurar|retomar|desfazer)/, 'undo'],
        [/^gerar novo/, 'refresh'],
        [/(mapa|localizar)/, 'pin'],
        [/^(suspender|interromper|pausar)/, 'pause'],
        [/(senha|segredo|revelar)/, 'key'],
        [/^sair/, 'logout'],
        [/^(importar|ligar as colunas|nova importacao)/, 'upload'],
        [/^(exportar|baixar|modelo)/, 'download'],
        [/(relatorio|erros|guia|artigos)/, 'file'],
        [/^(abrir|ver |ir para|todos|ver todas|mapeamentos)/, 'next']
    ];

    function normalizar(t) {
        return String(t).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/\s+/g, ' ').trim();
    }

    function svg(nome) {
        var d = ICONES[nome];
        if (!d) return null;
        var e = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        e.setAttribute('class', 'ico-btn');
        e.setAttribute('viewBox', '0 0 24 24');
        e.setAttribute('fill', 'none');
        e.setAttribute('stroke', 'currentColor');
        e.setAttribute('stroke-width', '1.9');
        e.setAttribute('stroke-linecap', 'round');
        e.setAttribute('stroke-linejoin', 'round');
        e.setAttribute('aria-hidden', 'true');
        e.innerHTML = d;
        return e;
    }

    function escolher(btn) {
        var manual = btn.getAttribute('data-icone');
        if (manual) return manual;
        var t = normalizar(btn.textContent);
        for (var i = 0; i < REGRAS.length; i++) {
            if (REGRAS[i][0].test(t)) return REGRAS[i][1];
        }
        return '';
    }

    function enfeitar(btn) {
        if (btn._icone || btn.hasAttribute('data-sem-icone') || btn.querySelector('svg')) return;
        btn._icone = true;
        if (btn.classList.contains('btn-menu') || !btn.textContent.trim()) return;
        var e = svg(escolher(btn));
        if (e) btn.insertBefore(e, btn.firstChild);
    }

    function varrer(raiz) {
        (raiz || document).querySelectorAll('.btn').forEach(enfeitar);
    }

    var pendente = false;
    function agendar() {
        if (pendente) return;
        pendente = true;
        requestAnimationFrame(function () {
            pendente = false;
            varrer(document);
        });
    }

    function iniciar() {
        varrer(document);
        new MutationObserver(function (registros) {
            var novo = registros.some(function (m) {
                return m.addedNodes.length > 0;
            });
            if (novo) agendar();
        }).observe(document.body, { childList: true, subtree: true });
    }

    window.iconeSvg = svg;
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', iniciar);
    else iniciar();
})();
