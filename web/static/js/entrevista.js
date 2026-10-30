// Entrevistas: editor das perguntas (cadastro) e regras de exibição condicional (ficha da investigação).
(function () {
    'use strict';

    document.addEventListener('alpine:init', function () {
        Alpine.data('editorEntrevista', function () {
            return {
                campos: [],
                seq: 0,
                init: function () {
                    var bruto = [];
                    try { bruto = JSON.parse(this.$el.getAttribute('data-campos') || '[]') || []; } catch (e) { bruto = []; }
                    this.campos = bruto.map(this.completar.bind(this));
                },
                completar: function (c) {
                    return {
                        _k: ++this.seq, nome: c.nome || '', label: c.label || '', tipo: c.tipo || 'texto', opcoes: c.opcoes || '',
                        obrigatorio: !!c.obrigatorio, placeholder: c.placeholder || '', cond_campo: c.cond_campo || '',
                        cond_operador: c.cond_operador || 'igual', cond_valor: c.cond_valor || '', exibir_se_agravo: c.exibir_se_agravo || ''
                    };
                },
                adicionar: function () { this.campos.push(this.completar({})); },
                remover: function (i) { this.campos.splice(i, 1); },
                mover: function (i, d) {
                    var j = i + d;
                    if (j < 0 || j >= this.campos.length) return;
                    var t = this.campos[i]; this.campos[i] = this.campos[j]; this.campos[j] = t;
                    this.campos = this.campos.slice();
                },
                // perguntas que podem condicionar a pergunta i (as outras que já têm identificador)
                outras: function (i) {
                    var self = this;
                    return this.campos.map(function (c, k) {
                        return { nome: self.identificador(c, k), label: c.label || ('Pergunta ' + (k + 1)) };
                    }).filter(function (_, k) { return k !== i; });
                },
                identificador: function (c, k) {
                    if (c.nome) return c.nome;
                    var t = (c.label || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
                    if (!t || !/^[a-z]/.test(t)) t = 'p_' + t;
                    return t.slice(0, 50).replace(/_+$/, '');
                },
                preparar: function () {
                    var self = this;
                    this.campos.forEach(function (c, k) { if (!c.nome) c.nome = self.identificador(c, k); });
                    this.$refs.saida.value = JSON.stringify(this.campos.map(function (c) {
                        var o = Object.assign({}, c); delete o._k; return o;
                    }));
                }
            };
        });
    });

    // Ficha da investigação: cada pergunta traz data-cond-campo/-op/-valor; ela só aparece quando a regra é cumprida.
    function valorDe(form, nome) {
        var els = form.querySelectorAll('[name="c_' + nome + '"]');
        var vals = [];
        els.forEach(function (el) {
            if ((el.type === 'checkbox' || el.type === 'radio') ? el.checked : true) vals.push(el.value);
        });
        return vals;
    }

    function atualizar(form) {
        form.querySelectorAll('[data-cond-campo]').forEach(function (bloco) {
            var campo = bloco.getAttribute('data-cond-campo');
            if (!campo) return;
            var bate = valorDe(form, campo).indexOf(bloco.getAttribute('data-cond-valor')) >= 0;
            var visivel = bloco.getAttribute('data-cond-op') === 'diferente' ? !bate : bate;
            bloco.hidden = !visivel;
            bloco.querySelectorAll('input, select, textarea').forEach(function (el) { el.disabled = !visivel; });
        });
    }

    function iniciar(raiz) {
        (raiz || document).querySelectorAll('form[data-entrevista]').forEach(function (form) {
            if (form._ent) return;
            form._ent = true;
            form.addEventListener('change', function () { atualizar(form); });
            form.addEventListener('input', function () { atualizar(form); });
            atualizar(form);
        });
    }

    document.addEventListener('htmx:load', function (e) { iniciar(e.target); });
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', function () { iniciar(document); });
    else iniciar(document);
})();
