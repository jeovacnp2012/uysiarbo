// Banners: editor visual (texto posicionado sobre cor ou imagem de fundo) e exportação em PNG.
(function () {
    'use strict';

    var FAMILIAS = {
        sans: 'system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif',
        serif: 'Georgia, "Times New Roman", serif',
        mono: 'ui-monospace, Consolas, "Courier New", monospace'
    };

    function limitar(v, a, b) { return Math.min(b, Math.max(a, v)); }

    document.addEventListener('alpine:init', function () {
        Alpine.data('editorBanner', function () {
            return {
                els: [],
                sel: -1,
                seq: 0,
                largura: 1080,
                altura: 1080,
                cor: '#155a9e',
                fundoURL: '',
                removerFundo: false,
                init: function () {
                    var el = this.$el, bruto = [];
                    try { bruto = JSON.parse(el.getAttribute('data-def') || '[]') || []; } catch (e) { bruto = []; }
                    this.largura = parseInt(el.getAttribute('data-largura'), 10) || 1080;
                    this.altura = parseInt(el.getAttribute('data-altura'), 10) || 1080;
                    this.cor = el.getAttribute('data-cor') || '#155a9e';
                    this.fundoURL = el.getAttribute('data-fundo') || '';
                    this.els = bruto.map(this.completar.bind(this));
                },
                completar: function (e) {
                    return {
                        _k: ++this.seq, texto: e.texto || '', x: typeof e.x === 'number' ? e.x : 10, y: typeof e.y === 'number' ? e.y : 10,
                        fonte: e.fonte || 'sans', tamanho: e.tamanho || 64, cor: e.cor || '#ffffff', negrito: !!e.negrito, sombra: !!e.sombra,
                        alinhar: e.alinhar || 'left'
                    };
                },
                adicionar: function () {
                    this.els.push(this.completar({ texto: 'Novo texto', x: 10, y: 10 + 8 * this.els.length }));
                    this.sel = this.els.length - 1;
                },
                remover: function (i) { this.els.splice(i, 1); this.sel = -1; },
                escolherFundo: function (ev) {
                    var f = ev.target.files && ev.target.files[0];
                    if (this.fundoURL.indexOf('blob:') === 0) URL.revokeObjectURL(this.fundoURL);
                    this.fundoURL = f ? URL.createObjectURL(f) : '';
                    this.removerFundo = false;
                },
                mostraFundo: function () { return this.fundoURL && !this.removerFundo; },
                estiloPalco: function () {
                    var s = 'aspect-ratio:' + (this.largura || 1) + '/' + (this.altura || 1) + ';background-color:' + this.cor + ';';
                    if (this.mostraFundo()) s += 'background-image:url("' + this.fundoURL + '");background-size:cover;background-position:center;';
                    return s;
                },
                // tamanho do texto em % da largura do banner (cqw), para a prévia acompanhar o tamanho real
                estiloTexto: function (e) {
                    var tx = e.alinhar === 'center' ? '-50%' : (e.alinhar === 'right' ? '-100%' : '0');
                    return 'left:' + e.x + '%;top:' + e.y + '%;transform:translateX(' + tx + ');text-align:' + e.alinhar + ';color:' + e.cor +
                        ';font-family:' + FAMILIAS[e.fonte] + ';font-size:' + (e.tamanho / (this.largura || 1) * 100) + 'cqw;font-weight:' + (e.negrito ? 700 : 400) +
                        ';text-shadow:' + (e.sombra ? '0.06em 0.06em 0.12em rgba(0,0,0,.65)' : 'none') + ';';
                },
                arrastar: function (ev, i) {
                    var self = this, palco = this.$refs.palco, e = this.els[i];
                    if (ev.button !== undefined && ev.button !== 0) return;
                    this.sel = i;
                    ev.preventDefault();
                    var r = palco.getBoundingClientRect();
                    var dx = ev.clientX - (r.left + e.x / 100 * r.width), dy = ev.clientY - (r.top + e.y / 100 * r.height);
                    function mover(m) {
                        e.x = Math.round(limitar((m.clientX - dx - r.left) / r.width * 100, 0, 100) * 10) / 10;
                        e.y = Math.round(limitar((m.clientY - dy - r.top) / r.height * 100, 0, 100) * 10) / 10;
                    }
                    function soltar() {
                        window.removeEventListener('pointermove', mover);
                        window.removeEventListener('pointerup', soltar);
                    }
                    window.addEventListener('pointermove', mover);
                    window.addEventListener('pointerup', soltar);
                    void self;
                },
                // grava a definição no campo escondido (só os textos preenchidos)
                preparar: function () {
                    var saida = this.els.filter(function (e) { return (e.texto || '').trim() !== ''; }).map(function (e) {
                        return { texto: e.texto, x: Number(e.x), y: Number(e.y), fonte: e.fonte, tamanho: parseInt(e.tamanho, 10) || 64, cor: e.cor, negrito: !!e.negrito, sombra: !!e.sombra, alinhar: e.alinhar };
                    });
                    this.$refs.saida.value = JSON.stringify(saida);
                },
                baixar: function () {
                    var self = this, c = document.createElement('canvas');
                    c.width = this.largura; c.height = this.altura;
                    var g = c.getContext('2d');
                    g.fillStyle = this.cor; g.fillRect(0, 0, c.width, c.height);
                    function textos() {
                        self.els.forEach(function (e) {
                            if (!(e.texto || '').trim()) return;
                            g.save();
                            g.font = (e.negrito ? '700 ' : '400 ') + e.tamanho + 'px ' + FAMILIAS[e.fonte];
                            g.fillStyle = e.cor; g.textAlign = e.alinhar; g.textBaseline = 'top';
                            if (e.sombra) { g.shadowColor = 'rgba(0,0,0,.65)'; g.shadowOffsetX = g.shadowOffsetY = e.tamanho * 0.06; g.shadowBlur = e.tamanho * 0.12; }
                            var x = e.x / 100 * c.width, y = e.y / 100 * c.height;
                            e.texto.split('\n').forEach(function (linha, k) { g.fillText(linha, x, y + k * e.tamanho * 1.2); });
                            g.restore();
                        });
                        var a = document.createElement('a');
                        a.download = (document.getElementById('nome').value || 'banner') + '.png';
                        c.toBlob(function (b) { a.href = URL.createObjectURL(b); a.click(); setTimeout(function () { URL.revokeObjectURL(a.href); }, 1000); });
                    }
                    if (this.mostraFundo()) {
                        var img = new Image();
                        img.onload = function () {
                            var k = Math.max(c.width / img.width, c.height / img.height), w = img.width * k, h = img.height * k;
                            g.drawImage(img, (c.width - w) / 2, (c.height - h) / 2, w, h);
                            textos();
                        };
                        img.onerror = textos;
                        img.src = this.fundoURL;
                    } else {
                        textos();
                    }
                }
            };
        });
    });
})();
