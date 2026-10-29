// Botões "Buscar pelo CEP" e "Localizar no mapa" do formulário do local.
// Chamam /locais/cep e /locais/geocodificar (ViaCEP e Nominatim, pelo servidor) e preenchem os campos.
(function () {
  var btnCEP = document.getElementById("btn-cep");
  var btnPos = document.getElementById("btn-localizar");
  var status = document.getElementById("geo-status");
  if (!btnCEP && !btnPos) return;

  function campo(id) { return document.getElementById(id); }
  function dizer(texto, erro) {
    if (!status) return;
    status.textContent = texto;
    status.className = "subtitulo" + (erro ? " texto-erro" : "");
  }
  function chamar(caminho, params, botao) {
    botao.disabled = true;
    dizer("Procurando...");
    return fetch(caminho + "?" + new URLSearchParams(params).toString(), { credentials: "same-origin", headers: { Accept: "application/json" } })
      .then(function (r) { return r.json().catch(function () { return { mensagem: "Resposta inesperada do servidor." }; }); })
      .catch(function () { return { mensagem: "Sem conexão com o servidor. Tente de novo." }; })
      .then(function (d) { botao.disabled = false; return d; });
  }

  if (btnCEP) btnCEP.addEventListener("click", function () {
    var cep = (campo("cep").value || "").trim();
    if (!cep) { dizer("Digite o CEP primeiro.", true); return; }
    chamar("/locais/cep", { cep: cep }, btnCEP).then(function (d) {
      if (!d.encontrado) { dizer(d.mensagem || "Não encontrei esse CEP.", true); return; }
      var rua = campo("endereco");
      var partes = [];
      if (d.logradouro) {
        if (!rua.value.trim()) { rua.value = d.logradouro; partes.push("rua preenchida"); }
        else if (rua.value.trim().toLowerCase() !== d.logradouro.toLowerCase()) partes.push("o CEP é da rua \"" + d.logradouro + "\"");
      }
      var sel = campo("bairro_id");
      if (d.bairro_id && sel && sel.value !== String(d.bairro_id)) {
        sel.value = String(d.bairro_id);
        if (window.htmx) window.htmx.trigger(sel, "change");
        partes.push("bairro " + d.bairro + " escolhido (confira o quarteirão)");
      } else if (d.bairro && !d.bairro_id) {
        partes.push("bairro \"" + d.bairro + "\" não está cadastrado");
      }
      dizer("CEP de " + d.cidade + "/" + d.uf + (partes.length ? ": " + partes.join("; ") : "") + ".");
    });
  });

  if (btnPos) btnPos.addEventListener("click", function () {
    var bairro = campo("bairro_id");
    chamar("/locais/geocodificar", {
      endereco: campo("endereco").value, numero: campo("numero").value, bairro_id: bairro ? bairro.value : "", cep: campo("cep").value,
    }, btnPos).then(function (d) {
      if (!d.encontrado) { dizer(d.mensagem || "Não consegui localizar.", true); return; }
      campo("latitude").value = d.latitude.toFixed(6);
      campo("longitude").value = d.longitude.toFixed(6);
      var msg = d.precisao === "endereco" ? "Posição do endereço encontrada." : "Posição aproximada (só a rua, o número não foi achado no mapa). Ajuste se precisar.";
      if (d.dentro_do_bairro === false) msg += " Atenção: o ponto está fora do limite gravado deste bairro.";
      dizer(msg + " Confira no mapa antes de salvar.", d.dentro_do_bairro === false);
    });
  });
})();
