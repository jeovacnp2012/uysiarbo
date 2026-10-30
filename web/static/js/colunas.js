// Colunas configuráveis e ordenação pelo título. Tabela: <table data-colunas="chave"> com th/td [data-col];
// th[data-fixa] nunca esconde. Menu: <details data-colunas-menu="chave">. Título ordenável: <a data-ordenar data-dir>,
// que preenche os campos ocultos "ordem" e "dir" do formulário de filtros vivo e dispara "filtrar".
(function () {
    'use strict';

    function ler(chave) {
        try { return JSON.parse(localStorage.getItem('uysi-colunas:' + chave) || '[]'); } catch (e) { return []; }
    }

    function gravar(chave, lista) {
        try { localStorage.setItem('uysi-colunas:' + chave, JSON.stringify(lista)); } catch (e) { /* sem armazenamento */ }
    }

    function aplicar(tabela) {
        var escondidas = ler(tabela.getAttribute('data-colunas'));
        tabela.querySelectorAll('[data-col]').forEach(function (el) {
            el.hidden = escondidas.indexOf(el.getAttribute('data-col')) >= 0;
        });
    }

    function montarMenu(menu) {
        var chave = menu.getAttribute('data-colunas-menu');
        var secao = menu.closest('section, .cartao') || document;
        var tabela = secao.querySelector('table[data-colunas="' + chave + '"]');
        var lista = menu.querySelector('.colunas-lista');
        if (!tabela || !lista || lista.children.length) return;
        var escondidas = ler(chave);
        tabela.querySelectorAll('thead th[data-col]').forEach(function (th) {
            var col = th.getAttribute('data-col');
            var l = document.createElement('label');
            var c = document.createElement('input');
            c.type = 'checkbox';
            c.checked = escondidas.indexOf(col) < 0;
            c.addEventListener('change', function () {
                var atual = ler(chave).filter(function (x) { return x !== col; });
                if (!c.checked) atual.push(col);
                gravar(chave, atual);
                aplicar(tabela);
            });
            l.appendChild(c);
            l.appendChild(document.createTextNode(' ' + th.textContent.replace(/[▲▼]/g, '').trim()));
            lista.appendChild(l);
        });
    }

    function varrer(raiz) {
        (raiz || document).querySelectorAll('table[data-colunas]').forEach(aplicar);
        (raiz || document).querySelectorAll('[data-colunas-menu]').forEach(montarMenu);
    }

    document.addEventListener('click', function (e) {
        var a = e.target.closest && e.target.closest('a[data-ordenar]');
        if (a) {
            e.preventDefault();
            var form = document.querySelector('form.filtros-vivos');
            if (!form || !form.elements.ordem) return;
            form.elements.ordem.value = a.getAttribute('data-ordenar');
            form.elements.dir.value = a.getAttribute('data-dir') || 'asc';
            document.body.dispatchEvent(new Event('filtrar'));
            return;
        }
        document.querySelectorAll('details.colunas-menu[open]').forEach(function (d) {
            if (!d.contains(e.target)) d.removeAttribute('open');
        });
    });
    document.addEventListener('htmx:load', function (e) { varrer(e.target); });
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', function () { varrer(document); });
    else varrer(document);
})();
