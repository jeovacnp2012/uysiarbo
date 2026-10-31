// B.I. Epidemiológico: abas, gráficos (ApexCharts) e filtro do cruzamento por bairro.
(function () {
	var raiz = document.getElementById("bie");
	var el = document.getElementById("bie-dados");
	if (!raiz || !el) return;
	var g = JSON.parse(el.textContent);
	var escuro = document.documentElement.dataset.tema === "escuro";
	var texto = escuro ? "#b0bccf" : "#46536a";
	var azul = escuro ? "#3987e5" : "#2a78d6", laranja = escuro ? "#d95926" : "#eb6834", verde = escuro ? "#199e70" : "#1baf7a";
	var anos = { 0: "#eda100", 1: "#6250d6", 2: "#1baf7a", 3: "#2a78d6" };
	var base = {
		chart: { fontFamily: "inherit", foreColor: texto, toolbar: { show: false }, animations: { speed: 350 } },
		theme: { mode: escuro ? "dark" : "light" },
		grid: { borderColor: escuro ? "#1d2839" : "#e7ebf2" },
		noData: { text: "Sem dados no filtro" },
		legend: { position: "top", horizontalAlign: "left" },
	};
	var feitos = {};
	function mesclar(a, b) { return Object.assign({}, a, b, { chart: Object.assign({}, a.chart, b.chart) }); }
	function desenhar(id, opcoes) {
		var alvo = document.getElementById(id);
		if (!alvo) return null;
		var c = new ApexCharts(alvo, mesclar(base, opcoes));
		c.render();
		return c;
	}
	var vazio = function (v) { return !v || !v.length; };

	function sparkResumo() {
		desenhar("bie-spark", {
			series: [{ name: "Investigações", data: g.semTotal || [] }],
			chart: { type: "area", height: 130, sparkline: { enabled: true }, foreColor: "#cfe0f3" },
			stroke: { curve: "smooth", width: 2.5, colors: ["#9fd0ff"] },
			fill: { type: "gradient", colors: ["#9fd0ff"], gradient: { opacityFrom: 0.45, opacityTo: 0.02 } },
			tooltip: { theme: "dark", x: { formatter: function (v, o) { return (g.semanas || [])[o.dataPointIndex] || ""; } } },
			theme: { mode: "dark" },
		});
	}
	function semana1() {
		desenhar("bie-sem1", {
			series: [{ name: "Confirmados", data: g.semConf || [] }, { name: "Descartados", data: g.semDesc || [] }, { name: "Outros", data: g.semOutros || [] }],
			chart: { type: "bar", height: 310, stacked: true },
			colors: [azul, "#9fb4d3", "#d6dde9"],
			plotOptions: { bar: { columnWidth: "78%", borderRadius: 2 } },
			xaxis: { categories: g.semanas || [], labels: { rotate: -60, style: { fontSize: "10px" } }, tickAmount: 14 },
			dataLabels: { enabled: false },
		});
	}
	function curva() {
		desenhar("bie-curva", {
			series: [{ name: "Investigações", type: "column", data: g.semTotal || [] }, { name: "Confirmados", type: "line", data: g.semConf || [] }],
			chart: { type: "line", height: 340, toolbar: { show: true, tools: { download: true, selection: false, zoom: false, zoomin: false, zoomout: false, pan: false, reset: false } } },
			colors: ["#a9c9f3", azul],
			stroke: { width: [0, 3], curve: "smooth" },
			plotOptions: { bar: { columnWidth: "72%", borderRadius: 2 } },
			xaxis: { categories: g.semanas || [], labels: { rotate: -60, style: { fontSize: "10px" } }, tickAmount: 14 },
			dataLabels: { enabled: false },
		});
	}
	function bairros() {
		desenhar("bie-bairros", {
			series: [{ name: "Investigações", data: g.bairroTotal || [] }, { name: "Confirmados", data: g.bairroConf || [] }],
			chart: { type: "bar", height: Math.max(300, (g.bairros || []).length * 36 + 80) },
			colors: ["#a9c9f3", azul],
			plotOptions: { bar: { horizontal: true, borderRadius: 4, barHeight: "70%" } },
			xaxis: { categories: g.bairros || [] },
			dataLabels: { enabled: true, style: { fontSize: "11px" } },
		});
	}
	function resultado() {
		var tot = (g.semConf || []).reduce(function (a, b) { return a + b; }, 0);
		var desc = (g.semDesc || []).reduce(function (a, b) { return a + b; }, 0);
		var out = (g.semOutros || []).reduce(function (a, b) { return a + b; }, 0);
		desenhar("bie-resultado", {
			series: [tot, desc, out], labels: ["Confirmados", "Descartados", "Inconclusivos e em investigação"],
			chart: { type: "donut", height: 300 },
			colors: [azul, "#9fb4d3", "#d6dde9"],
			plotOptions: { pie: { donut: { size: "64%", labels: { show: true, total: { show: true, label: "Total", color: texto } } } } },
			legend: { position: "bottom" },
			stroke: { colors: [escuro ? "#111925" : "#fff"] },
		});
	}
	function mesmoPeriodo() {
		desenhar("bie-hmp", {
			series: [{ name: "Investigações", data: g.anosTotal || [] }, { name: "Confirmados", data: g.anosConf || [] }],
			chart: { type: "bar", height: 310 },
			colors: ["#a9c9f3", azul],
			plotOptions: { bar: { columnWidth: "55%", borderRadius: 5, dataLabels: { position: "top" } } },
			dataLabels: { enabled: true, offsetY: -18, style: { fontSize: "11px", colors: [texto] } },
			xaxis: { categories: g.anosRotulos || [] },
		});
	}
	var meses = ["Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"], graficoMes = null, modoMes = "conf";
	function curvaMensal() {
		var chaves = Object.keys(g.mesesAnos || {}).sort();
		var series = chaves.map(function (a) { return { name: a, data: (g.mesesAnos[a] || {})[modoMes] || [] }; });
		var cores = chaves.map(function (a, i) { return anos[i + (4 - chaves.length)]; });
		if (graficoMes) graficoMes.destroy();
		graficoMes = desenhar("bie-hmes", {
			series: series, chart: { type: "line", height: 310 }, colors: cores,
			stroke: { width: 3, curve: "smooth" }, markers: { size: 3 },
			xaxis: { categories: meses }, dataLabels: { enabled: false },
		});
	}
	function evolucao() {
		desenhar("bie-evol", {
			series: [{ name: "Fechados sem recuperação (%)", data: g.cicloSemRec || [] }, { name: "Recuperação dos fechados (%)", data: g.cicloRecup || [] }],
			chart: { type: "line", height: 280 }, colors: [laranja, verde],
			stroke: { width: 3, curve: "smooth" }, markers: { size: 4 },
			xaxis: { categories: g.cicloRotulos || [], labels: { rotate: -30, style: { fontSize: "11px" } } },
			yaxis: { min: 0, max: 100, labels: { formatter: function (v) { return v + "%"; } } },
			dataLabels: { enabled: false },
			tooltip: { y: { formatter: function (v) { return v + "%"; } } },
		});
		desenhar("bie-imoveis", {
			series: [{ name: "Imóveis visitados", data: g.cicloImoveis || [] }],
			chart: { type: "bar", height: 280 }, colors: [laranja],
			plotOptions: { bar: { columnWidth: "55%", borderRadius: 5 } },
			xaxis: { categories: g.cicloRotulos || [], labels: { rotate: -30, style: { fontSize: "11px" } } },
			dataLabels: { enabled: true, style: { fontSize: "11px" } },
		});
	}
	function pendencias() {
		desenhar("bie-pend", {
			series: [{ name: "Sem recuperação (%)", data: g.pendValores || [] }],
			chart: { type: "bar", height: Math.max(280, (g.pendBairros || []).length * 32 + 70) }, colors: [laranja],
			plotOptions: { bar: { horizontal: true, borderRadius: 4, barHeight: "70%" } },
			xaxis: { categories: g.pendBairros || [], labels: { formatter: function (v) { return v + "%"; } } },
			dataLabels: { enabled: true, formatter: function (v) { return v + "%"; }, style: { fontSize: "11px" } },
			tooltip: { y: { formatter: function (v) { return v + "%"; } } },
		});
	}
	function historicoCampo() {
		desenhar("bie-hc-sem", {
			series: [{ name: "Fechados sem recuperação (%)", data: g.hCampoSemRec || [] }],
			chart: { type: "bar", height: 280 }, colors: [laranja],
			plotOptions: { bar: { columnWidth: "50%", borderRadius: 5 } },
			xaxis: { categories: g.anosRotulos || [] },
			yaxis: { labels: { formatter: function (v) { return v + "%"; } } },
			dataLabels: { enabled: true, formatter: function (v) { return v + "%"; }, style: { fontSize: "11px" } },
		});
		desenhar("bie-hc-rec", {
			series: [{ name: "Recuperação dos fechados (%)", data: g.hCampoRecup || [] }],
			chart: { type: "bar", height: 280 }, colors: [verde],
			plotOptions: { bar: { columnWidth: "50%", borderRadius: 5 } },
			xaxis: { categories: g.anosRotulos || [] },
			yaxis: { max: 100, labels: { formatter: function (v) { return v + "%"; } } },
			dataLabels: { enabled: true, formatter: function (v) { return v + "%"; }, style: { fontSize: "11px" } },
		});
	}
	function dispersao() {
		var pts = (g.pontos || []).map(function (p) { return { x: p.x, y: p.y, nome: p.nome }; });
		desenhar("bie-scatter", {
			series: [{ name: "Bairros", data: pts }],
			chart: { type: "scatter", height: 400, zoom: { enabled: false } }, colors: [azul],
			markers: { size: 8, strokeWidth: 0 },
			xaxis: { title: { text: "Investigações no bairro" }, tickAmount: 8, labels: { formatter: function (v) { return Math.round(v); } } },
			yaxis: { title: { text: "Fechados sem recuperação (%)" }, min: 0, labels: { formatter: function (v) { return v.toFixed(0) + "%"; } } },
			tooltip: { custom: function (o) { var p = o.w.config.series[o.seriesIndex].data[o.dataPointIndex]; return '<div style="padding:8px 10px"><b>' + p.nome.replace(/</g, "&lt;") + "</b><br>" + p.x + " investigações · " + p.y + "% sem recuperação</div>"; } },
		});
	}
	var abas = {
		resumo: [sparkResumo, semana1], epidemiologia: [curva, bairros, resultado], historico: [mesmoPeriodo, curvaMensal],
		entomologia: [evolucao, pendencias], hcampo: [historicoCampo], cruzamento: [dispersao], notas: [],
	};
	function mostrar(nome) {
		if (!abas[nome]) nome = "resumo";
		raiz.querySelectorAll(".bie-view").forEach(function (s) { s.classList.toggle("on", s.id === "bie-" + nome); });
		raiz.querySelectorAll(".bie-abas button").forEach(function (b) { b.setAttribute("aria-pressed", b.dataset.aba === nome ? "true" : "false"); });
		if (!feitos[nome]) {
			feitos[nome] = true;
			abas[nome].forEach(function (f) { try { f(); } catch (e) { console.error(e); } });
		}
		window.dispatchEvent(new Event("resize"));
	}
	raiz.querySelectorAll(".bie-abas button").forEach(function (b) {
		b.addEventListener("click", function () { history.replaceState(null, "", "#" + b.dataset.aba); mostrar(b.dataset.aba); });
	});
	var bm = document.getElementById("bie-hm-c"), bn = document.getElementById("bie-hm-n");
	function modo(m) { modoMes = m; bm.setAttribute("aria-pressed", m === "conf"); bn.setAttribute("aria-pressed", m === "total"); curvaMensal(); }
	if (bm && bn) { bm.addEventListener("click", function () { modo("conf"); }); bn.addEventListener("click", function () { modo("total"); }); }

	// filtro da tabela de bairros
	var fc = document.getElementById("bie-f-casos"), fp = document.getElementById("bie-f-pend"), fq = document.getElementById("bie-f-q"), cont = document.getElementById("bie-f-count");
	function filtrar() {
		var minC = +fc.value || 0, minP = +fp.value || 0, q = (fq.value || "").toLowerCase(), n = 0, tot = 0;
		document.querySelectorAll("#bie-t-bairros tbody tr").forEach(function (tr) {
			tot++;
			var pend = parseFloat(tr.dataset.pend);
			var ok = +tr.dataset.casos >= minC && (minP === 0 || pend >= minP) && tr.dataset.nome.toLowerCase().indexOf(q) >= 0;
			tr.style.display = ok ? "" : "none";
			if (ok) n++;
		});
		if (cont) cont.textContent = n + " de " + tot + " bairros";
	}
	[fc, fp, fq].forEach(function (i) { if (i) i.addEventListener("input", filtrar); });
	filtrar();

	mostrar(location.hash.slice(1) || "resumo");
})();
