// B.I.: gráficos com ApexCharts, no mesmo estilo do Geo Controle (barras, rosquinhas e medidor).
(function () {
	var el = document.getElementById("bi-dados");
	if (!el || !window.ApexCharts) return;
	var g = JSON.parse(el.textContent);
	var escuro = document.documentElement.dataset.tema === "escuro";
	var texto = escuro ? "#cbd5e1" : "#475569";
	var base = {
		chart: { fontFamily: "inherit", foreColor: texto, toolbar: { show: true, tools: { download: true, selection: false, zoom: false, zoomin: false, zoomout: false, pan: false, reset: false } }, animations: { speed: 400 } },
		theme: { mode: escuro ? "dark" : "light" },
		grid: { borderColor: escuro ? "#334155" : "#e2e8f0" },
		noData: { text: "Sem dados no filtro" },
	};
	function mesclar(a, b) { return Object.assign({}, a, b, { chart: Object.assign({}, a.chart, b.chart) }); }
	function desenhar(id, opcoes) {
		var alvo = document.getElementById(id);
		if (!alvo) return;
		new ApexCharts(alvo, mesclar(base, opcoes)).render();
	}
	var soma = function (v) { return v.reduce(function (a, b) { return a + b; }, 0); };
	var paleta = ["#2563eb", "#16a34a", "#f59e0b", "#dc2626", "#7c3aed", "#0891b2", "#db2777", "#65a30d", "#ea580c", "#475569", "#94a3b8"];

	desenhar("bi-visitas", {
		series: [{ name: "Visitas", data: g.visitas || [] }],
		chart: { type: "bar", height: 380 },
		plotOptions: { bar: { borderRadius: 6, dataLabels: { position: "top" } } },
		dataLabels: { enabled: true, offsetY: -20, style: { fontSize: "11px", colors: [texto] } },
		xaxis: { categories: g.visitasBairros || [], labels: { rotate: -45, rotateAlways: true, style: { fontSize: "11px" } } },
		yaxis: { title: { text: "Quantidade de visitas" } },
		colors: ["#2563eb"],
	});
	var tipos = ["A1", "A2", "B", "C", "D1", "D2", "E"], dep = g.depositos || [];
	desenhar("bi-depositos", {
		series: [{ name: "Quantidade", data: dep }],
		chart: { type: "bar", height: 380 },
		plotOptions: { bar: { borderRadius: 6, horizontal: true, distributed: true } },
		dataLabels: { enabled: true, formatter: function (v) { var t = soma(dep); return v + " (" + (t ? (v * 100 / t).toFixed(1) : 0) + "%)"; }, style: { fontSize: "11px" } },
		xaxis: { categories: tipos },
		legend: { show: false },
		colors: ["#ef4444", "#3b82f6", "#f59e0b", "#14b8a6", "#8b5cf6", "#f97316", "#22c55e"],
	});
	function rosca(id, rotulos, valores, extra) {
		desenhar(id, Object.assign({
			series: valores, labels: rotulos, colors: paleta,
			chart: { type: "donut", height: 380 },
			plotOptions: { pie: { donut: { size: "65%", labels: { show: true, total: { show: true, label: "Total", color: texto, formatter: function (w) { return soma(w.globals.seriesTotals); } } } } } },
			dataLabels: { enabled: true, formatter: function (v, o) { return o.w.config.series[o.seriesIndex]; } },
			legend: { position: "bottom", fontSize: "12px" },
			stroke: { colors: [escuro ? "#1e293b" : "#fff"] },
		}, extra || {}));
	}
	rosca("bi-positivos", g.posBairros || [], g.posBairro || [], { tooltip: { y: { formatter: function (v) { return v + " locais"; } } } });
	desenhar("bi-cobertura", {
		series: [{ name: "Cobertura", data: g.cobertura || [] }],
		chart: { type: "bar", height: 380 },
		plotOptions: { bar: { borderRadius: 6, dataLabels: { position: "top" } } },
		dataLabels: { enabled: true, formatter: function (v) { return v + "%"; }, offsetY: -20, style: { fontSize: "10px", colors: [texto] } },
		xaxis: { categories: g.cobBairros || [], labels: { rotate: -45, rotateAlways: true, style: { fontSize: "11px" } } },
		yaxis: { title: { text: "Cobertura (%)" }, max: 100 },
		tooltip: { y: { formatter: function (v) { return v + "%"; } } },
		colors: ["#16a34a"],
	});
	rosca("bi-amostras", ["Positivos", "Negativos"], [g.amostrasPositivas || 0, g.amostrasNegativas || 0], { colors: ["#ef4444", "#22c55e"] });
	rosca("bi-tipos", g.tipoLocais || [], g.tipoValores || []);
	desenhar("bi-ciclo", {
		series: [g.coberturaCiclo || 0],
		chart: { type: "radialBar", height: 340, toolbar: { show: false } },
		plotOptions: { radialBar: { startAngle: -90, endAngle: 90, hollow: { margin: 0, size: "60%" }, track: { background: escuro ? "#334155" : "#e2e8f0", strokeWidth: "100%" }, dataLabels: { name: { show: false }, value: { fontSize: "44px", fontWeight: 700, offsetY: -4, color: texto, formatter: function (v) { return v + "%"; } } } } },
		fill: { type: "gradient", gradient: { shade: "dark", type: "horizontal", gradientToColors: ["#22c55e"], stops: [0, 100] } },
		stroke: { lineCap: "round" },
		colors: ["#2563eb"],
		labels: [""],
	});
})();
