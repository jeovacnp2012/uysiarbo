// Campo de seleção pesquisável: todo <select> com mais de 5 opções (sem contar a opção vazia)
// ganha uma caixa de pesquisa interna. O <select> original continua no formulário, escondido,
// e recebe os eventos input/change, então HTMX, Alpine e o envio do formulário seguem iguais.
(function () {
    'use strict';
    var LIMITE = 5;

    function normalizar(t) {
        return String(t).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
    }

    function quantas(sel) {
        var n = 0;
        for (var i = 0; i < sel.options.length; i++) {
            if (sel.options[i].value !== '') n++;
        }
        return n;
    }

    function textoSelecionado(sel) {
        var o = sel.options[sel.selectedIndex];
        return o ? o.textContent.trim() : '';
    }

    function el(tag, classe, atributos) {
        var e = document.createElement(tag);
        if (classe) e.className = classe;
        for (var k in atributos || {}) e.setAttribute(k, atributos[k]);
        return e;
    }

    function montar(sel) {
        var raiz = el('div', 'sb');
        var botao = el('button', 'sb-botao', { type: 'button', 'aria-haspopup': 'listbox', 'aria-expanded': 'false' });
        var texto = el('span', 'sb-texto');
        botao.appendChild(texto);
        botao.appendChild(el('span', 'sb-seta', { 'aria-hidden': 'true' }));
        var painel = el('div', 'sb-painel');
        painel.hidden = true;
        var busca = el('input', 'sb-busca', { type: 'search', placeholder: 'Pesquisar…', autocomplete: 'off', 'aria-label': 'Pesquisar na lista' });
        var lista = el('ul', 'sb-lista', { role: 'listbox' });
        var vazio = el('div', 'sb-vazio');
        vazio.textContent = 'Nenhum resultado';
        vazio.hidden = true;
        painel.appendChild(busca);
        painel.appendChild(lista);
        painel.appendChild(vazio);
        raiz.appendChild(botao);
        raiz.appendChild(painel);

        var itens = [];
        var ativo = -1;

        function rotular() {
            var t = textoSelecionado(sel);
            texto.textContent = t || ' ';
            texto.classList.toggle('sb-placeholder', sel.value === '');
        }

        function destacar(i) {
            if (ativo >= 0 && itens[ativo]) itens[ativo].li.classList.remove('sb-ativo');
            ativo = i;
            if (i >= 0 && itens[i]) {
                itens[i].li.classList.add('sb-ativo');
                itens[i].li.scrollIntoView({ block: 'nearest' });
            }
        }

        function filtrar() {
            var termo = normalizar(busca.value.trim());
            var primeiro = -1;
            var visiveis = 0;
            itens.forEach(function (it, i) {
                var ok = !termo || normalizar(it.rotulo).indexOf(termo) !== -1;
                it.li.hidden = !ok;
                if (ok) {
                    visiveis++;
                    if (primeiro < 0 && !it.desativado) primeiro = i;
                }
            });
            // cabeçalhos de grupo só aparecem se algum item do grupo aparece
            lista.querySelectorAll('.sb-grupo').forEach(function (g) {
                var prox = g.nextElementSibling;
                var tem = false;
                while (prox && !prox.classList.contains('sb-grupo')) {
                    if (!prox.hidden) tem = true;
                    prox = prox.nextElementSibling;
                }
                g.hidden = !tem;
            });
            vazio.hidden = visiveis > 0;
            destacar(primeiro);
        }

        function construir() {
            lista.textContent = '';
            itens = [];
            var grupoAtual = null;
            Array.prototype.forEach.call(sel.options, function (o, idx) {
                if (o.parentElement && o.parentElement.tagName === 'OPTGROUP' && o.parentElement !== grupoAtual) {
                    grupoAtual = o.parentElement;
                    var g = el('li', 'sb-grupo', { role: 'presentation' });
                    g.textContent = grupoAtual.label;
                    lista.appendChild(g);
                } else if (!o.parentElement || o.parentElement.tagName !== 'OPTGROUP') {
                    grupoAtual = null;
                }
                var li = el('li', 'sb-item', { role: 'option', 'aria-selected': o.selected ? 'true' : 'false' });
                var rotulo = o.textContent.trim();
                li.textContent = rotulo || ' ';
                if (o.value === '') li.classList.add('sb-placeholder');
                if (o.disabled) li.setAttribute('aria-disabled', 'true');
                if (o.selected) li.classList.add('sb-escolhido');
                var pos = itens.length;
                li.addEventListener('mousedown', function (ev) {
                    ev.preventDefault(); // mantém o foco na caixa de pesquisa até escolher
                });
                li.addEventListener('click', function () {
                    escolher(pos);
                });
                li.addEventListener('mousemove', function () {
                    if (ativo !== pos) destacar(pos);
                });
                lista.appendChild(li);
                itens.push({ li: li, indice: idx, rotulo: rotulo, desativado: o.disabled });
            });
        }

        function aberto() {
            return !painel.hidden;
        }

        function abrir() {
            if (sel.matches(':disabled') || aberto()) return;
            construir();
            busca.value = '';
            painel.hidden = false;
            botao.setAttribute('aria-expanded', 'true');
            filtrar();
            var atual = itens.findIndex(function (it) {
                return it.indice === sel.selectedIndex;
            });
            if (atual >= 0) destacar(atual);
            busca.focus();
        }

        function fechar(devolverFoco) {
            if (!aberto()) return;
            painel.hidden = true;
            botao.setAttribute('aria-expanded', 'false');
            if (devolverFoco) botao.focus();
        }

        function escolher(pos) {
            var it = itens[pos];
            if (!it || it.desativado) return;
            var mudou = sel.selectedIndex !== it.indice;
            sel.selectedIndex = it.indice;
            rotular();
            fechar(true);
            if (mudou) {
                sel.dispatchEvent(new Event('input', { bubbles: true }));
                sel.dispatchEvent(new Event('change', { bubbles: true }));
            }
        }

        function mover(passo) {
            var i = ativo;
            do {
                i += passo;
            } while (i >= 0 && i < itens.length && (itens[i].li.hidden || itens[i].desativado));
            if (i >= 0 && i < itens.length) destacar(i);
        }

        botao._sel = sel;
        botao.addEventListener('click', function () {
            if (aberto()) fechar(false);
            else abrir();
        });
        botao.addEventListener('keydown', function (ev) {
            if (ev.key === 'ArrowDown' || ev.key === 'ArrowUp') {
                ev.preventDefault();
                abrir();
            }
        });
        busca.addEventListener('input', filtrar);
        busca.addEventListener('keydown', function (ev) {
            if (ev.key === 'ArrowDown') {
                ev.preventDefault();
                mover(1);
            } else if (ev.key === 'ArrowUp') {
                ev.preventDefault();
                mover(-1);
            } else if (ev.key === 'Enter') {
                ev.preventDefault(); // não envia o formulário
                escolher(ativo);
            } else if (ev.key === 'Escape') {
                ev.preventDefault();
                ev.stopPropagation();
                fechar(true);
            } else if (ev.key === 'Tab') {
                fechar(false);
            }
        });
        document.addEventListener('mousedown', function (ev) {
            if (aberto() && !raiz.contains(ev.target)) fechar(false);
        });
        // clicar no <label for=...> ou focar o select por script leva o foco para o botão
        sel.addEventListener('focus', function () {
            botao.focus();
        });

        sel.classList.add('sb-nativo');
        sel.tabIndex = -1;
        sel.setAttribute('aria-hidden', 'true');
        sel.insertAdjacentElement('afterend', raiz);
        sel._sb = {
            raiz: raiz,
            botao: botao,
            atualizar: function () {
                rotular();
                var desativado = sel.matches(':disabled');
                botao.disabled = desativado;
                raiz.classList.toggle('sb-desativado', desativado);
                var id = sel.id;
                if (id) botao.id = id + '-sb';
                if (!sel.hasAttribute('aria-label')) {
                    var rot = id && document.querySelector('label[for="' + id + '"]');
                    if (rot) botao.setAttribute('aria-label', rot.textContent.trim());
                }
            },
        };
        sel._sb.atualizar();
    }

    function revisar(sel) {
        if (sel.multiple || sel.size > 1 || sel.hasAttribute('data-sem-busca')) return;
        var deve = quantas(sel) > LIMITE;
        if (deve && !sel._sb) montar(sel);
        if (sel._sb) {
            sel._sb.raiz.hidden = !deve;
            sel.classList.toggle('sb-nativo', deve);
            sel.tabIndex = deve ? -1 : 0;
            if (deve) sel._sb.atualizar();
        }
    }

    function varrer(raiz) {
        (raiz || document).querySelectorAll('select').forEach(revisar);
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
        // opções trocadas por HTMX/Alpine, selects novos e selects que deixaram de ter mais de 5 opções
        new MutationObserver(function (registros) {
            var relevante = registros.some(function (m) {
                var alvo = m.target.nodeType === 1 ? m.target : m.target.parentElement;
                return !(alvo && alvo.closest && alvo.closest('.sb'));
            });
            if (relevante) agendar();
        }).observe(document.body, { childList: true, subtree: true, characterData: true });
        document.addEventListener('htmx:afterSwap', agendar);
        // o valor pode ter sido trocado por script (ex.: Alpine x-model): confere o texto ao entrar no campo
        document.addEventListener('focusin', function (ev) {
            var s = ev.target && ev.target._sel;
            if (s && s._sb) s._sb.atualizar();
        });
    }

    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', iniciar);
    else iniciar();
})();
