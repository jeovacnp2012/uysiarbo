// Filtros em gaveta: todo formulário de filtros (GET) vira um painel que desliza da direita para a esquerda,
// com um botão "Filtros" acima da lista. O formulário continua no mesmo lugar do HTML (HTMX, mapa e relatórios
// seguem funcionando); só a apresentação muda. Para deixar um formulário de fora, use data-sem-gaveta; para abrir
// a gaveta já na chegada (ex.: relatório ainda não gerado), use data-gaveta-aberta.
(function () {
    'use strict';

    var IGNORAR = { csrf: 1, page: 1, por_pagina: 1, pagina: 1 };
    var fundo = null;
    var aberta = null;

    function garantirFundo() {
        if (fundo) return fundo;
        fundo = document.createElement('div');
        fundo.className = 'gaveta-fundo';
        fundo.addEventListener('click', fechar);
        document.body.appendChild(fundo);
        return fundo;
    }

    function ativos(form) {
        var n = 0;
        form.querySelectorAll('input[name], select[name], textarea[name]').forEach(function (el) {
            if (IGNORAR[el.name] || el.type === 'hidden' || el.type === 'submit' || el.type === 'button') return;
            if ((el.type === 'checkbox' || el.type === 'radio') ? el.checked : String(el.value).trim() !== '') n++;
        });
        return n;
    }

    function atualizarContagem(form) {
        var selo = form._gavetaBotao && form._gavetaBotao.querySelector('.gaveta-contagem');
        if (!selo) return;
        var n = ativos(form);
        selo.textContent = n;
        selo.hidden = n === 0;
    }

    function abrir(form) {
        if (aberta && aberta !== form) fechar();
        aberta = form;
        garantirFundo().classList.add('aberto');
        form.classList.add('aberta');
        form.setAttribute('aria-hidden', 'false');
        var primeiro = form.querySelector('input:not([type=hidden]), select, textarea');
        if (primeiro) setTimeout(function () { primeiro.focus({ preventScroll: true }); }, 220);
    }

    function fechar() {
        if (!aberta) return;
        aberta.classList.remove('aberta');
        aberta.setAttribute('aria-hidden', 'true');
        if (aberta._gavetaBotao) aberta._gavetaBotao.focus({ preventScroll: true });
        aberta = null;
        if (fundo) fundo.classList.remove('aberto');
    }

    function montar(form) {
        if (form._gaveta || form.hasAttribute('data-sem-gaveta')) return;
        form._gaveta = true;
        form.classList.add('gaveta');
        form.setAttribute('aria-hidden', 'true');

        var titulo = form.getAttribute('data-gaveta-titulo') || 'Filtros';
        var cab = document.createElement('div');
        cab.className = 'gaveta-cab';
        cab.innerHTML = '<strong class="gaveta-titulo" data-ico="sliders"></strong>' +
            '<button type="button" class="btn gaveta-fechar" data-icone="x" aria-label="Fechar filtros">Fechar</button>';
        cab.querySelector('.gaveta-titulo').appendChild(document.createTextNode(titulo));
        cab.querySelector('.gaveta-fechar').addEventListener('click', fechar);
        form.insertBefore(cab, form.firstChild);

        var barra = document.createElement('div');
        barra.className = 'gaveta-barra';
        barra.innerHTML = '<button type="button" class="btn gaveta-abrir" data-icone="sliders" aria-haspopup="dialog">' +
            titulo + ' <span class="gaveta-contagem selo" hidden></span></button>';
        var botao = barra.querySelector('button');
        botao.addEventListener('click', function () { abrir(form); });
        form._gavetaBotao = botao;
        form.parentNode.insertBefore(barra, form);

        form.addEventListener('input', function () { atualizarContagem(form); });
        form.addEventListener('change', function () { atualizarContagem(form); });
        atualizarContagem(form);
        if (form.hasAttribute('data-gaveta-aberta')) abrir(form);
    }

    function varrer(raiz) {
        (raiz || document).querySelectorAll('form.filtros[method="get" i], form.rel-form').forEach(montar);
    }

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') fechar();
    });
    document.addEventListener('htmx:load', function (e) { varrer(e.target); });

    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', function () { varrer(document); });
    else varrer(document);
})();
