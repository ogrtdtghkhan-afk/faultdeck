"use strict";

(() => {
  const words = {
    en: {
      skip: "Skip to content",
      docs: "Docs",
      source: "Source",
      eyebrow: "SELF-HOSTED API FAULT INJECTION",
      heroLineOne: "Break it here.",
      heroLineTwo: "Ship it stronger.",
      heroDescription:
        "Deploy a real proxy in front of your development API. Add delays, errors, and dropped connections—then check how your app recovers.",
      download: "Download FaultDeck",
      quickStartLink: "Get started",
      heroNote: "Docker Compose or a single executable · Free & open source",
      trustLine: "Your infrastructure. Persistent rules. Real API traffic.",
      sitePurpose: "This is the project overview. The working control panel runs in your Docker deployment or downloaded executable.",
      deployCta: "Deploy with Docker",
      binaryCta: "Download binary",
      deployTitle: "Run the actual product.",
      deployDescription: "A real proxy and control panel, on your own machine or server.",
      deployStepOne: "1. Get the project and configuration",
      deployPassword: "2. Edit .env and set FAULTDECK_ADMIN_PASSWORD to a password of at least 12 characters. In PowerShell, use Copy-Item instead of cp.",
      deployStepThree: "3. Start the service",
      controlPanel: "Control panel",
      deployLogin: "Sign in as admin with the password you configured. Set your backend URL in the control panel.",
      deploymentGuide: "Full deployment & server access guide",
      protectedAdmin: "Protected admin panel",
      persistentRules: "Persistent configuration",
      simulationEyebrow: "OPTIONAL BROWSER WALKTHROUGH",
      simulationTitle: "See one failure pattern.",
      simulationIntro: "This small simulation explains the “fail twice” rule. To intercept your own app’s requests, run FaultDeck with Docker Compose or the executable above.",
      backToSetup: "Connect your real backend",
      upstreamResponse: "upstream",
      interactiveDemo: "INTERACTIVE DEMO",
      demoTitle: "Fail twice. Then recover.",
      simulation: "SIMULATION",
      sampleApi: "sample API",
      scenarioRule: "Return 503 for the first 2 requests",
      requestOne: "First request",
      requestTwo: "Second request",
      requestThree: "Third request",
      waiting: "Waiting for you",
      reset: "Reset",
      simulationNote: "Browser simulation only. No API requests are sent.",
      latencyTitle: "Slow it down",
      latencyDescription: "Check loading states and client timeouts.",
      errorTitle: "Make it fail",
      errorDescription: "Exercise rate limits, errors, and retries.",
      disconnectTitle: "Cut the connection",
      disconnectDescription: "See what happens when no response arrives.",
      dashboardEyebrow: "THE REAL THING",
      dashboardTitle: "A control panel for the unexpected.",
      dashboardDescription:
        "Connect your backend, create a fault rule, and inspect real request results. Docker stores your target and rules across restarts.",
      localDashboard: "SELF-HOSTED DASHBOARD",
      screenshotCaption:
        "Actual FaultDeck dashboard. Two injected failures, followed by a successful response.",
      quickStartEyebrow: "CONNECT YOUR REAL APPLICATION",
      quickStartTitle: "Deploy. Connect. Test recovery.",
      quickStartDescription:
        "Point FaultDeck at your backend, then point your client at FaultDeck. The built-in demo is optional.",
      stepOneTitle: "Start your instance",
      stepOneDescription:
        "Use Docker Compose above, or extract a release and start the executable:",
      chooseDownload: "Choose your download",
      stepTwoTitle: "Connect your backend",
      stepTwoDescription:
        "Open your control panel. Set the upstream target to your real backend, reachable from the FaultDeck server.",
      stepTwoAside:
        "The initial target may be the built-in demo. Replace it with your service URL and save.",
      stepThreeTitle: "Send real app requests",
      stepThreeDescription:
        "Set your client’s base URL to the proxy address shown in the panel. Add X-FaultDeck-Token from your deployment configuration when required.",
      stepThreeAside:
        "Create a rule for your endpoint. Make real requests from your app, then inspect the activity log. A fail-twice rule lets the third request reach your backend.",
      closingEyebrow: "BUILT FOR THE UNHAPPY PATH",
      closingTitle: "Find the failure before your users do.",
      closingDescription:
        "Try a scenario. Tell us what broke. Help shape a better development tool.",
      viewGitHub: "View on GitHub",
      feedback: "Share feedback or an idea",
      footerTagline: "Small failures. Stronger software.",
      license: "MIT license",
      privacy: "No analytics. No cookies.",
      sending: "Simulating…",
      complete: "Recovered · 3 of 3",
      send: (number) => `Send request ${number}`,
      initialProgress: "Click to send the first simulated request.",
      busyProgress: (number) => `Simulating request ${number} of 3…`,
      firstProgress:
        "503 injected. Send another request to test the next attempt.",
      secondProgress: "Two faults injected. The next request can pass through.",
      finalProgress:
        "200 OK. The rule’s limit is reached, so the third request succeeds.",
      failedDetail: "Injected fault · Service unavailable",
      recoveredDetail: "Rule exhausted · Passed through",
      screenshotAlt:
        "FaultDeck dashboard after two injected 503 failures. The rule has reached its limit and the next request returns 200.",
      traceLabel: "Simulated request results",
      statusesLabel: "Fail-twice rule: 503, 503, then the upstream response",
      limit: "LIMIT 2",
    },
    zh: {
      skip: "跳转到正文",
      docs: "文档",
      source: "源码",
      eyebrow: "可自行部署的 API 故障工具",
      heroLineOne: "在这里制造故障。",
      heroLineTwo: "让上线更有底气。",
      heroDescription:
        "在你的开发接口前部署一个真实代理。注入延迟、错误和连接中断，检查应用能否正确恢复。",
      download: "下载 FaultDeck",
      quickStartLink: "开始使用",
      heroNote: "Docker Compose 或单个可执行文件 · 免费开源",
      trustLine: "自行部署，规则持久保存，处理真实 API 请求。",
      sitePurpose: "这里是项目介绍页。真正的控制面板运行在你部署的 Docker 服务或下载的可执行程序中。",
      deployCta: "使用 Docker 部署",
      binaryCta: "下载可执行程序",
      deployTitle: "运行真正的产品。",
      deployDescription: "在自己的电脑或服务器上运行代理和控制面板。",
      deployStepOne: "1. 获取项目和配置文件",
      deployPassword: "2. 编辑 .env，将 FAULTDECK_ADMIN_PASSWORD 设置为至少 12 个字符的密码。PowerShell 中请用 Copy-Item 复制配置文件。",
      deployStepThree: "3. 启动服务",
      controlPanel: "控制面板",
      deployLogin: "使用 admin 和自己设置的密码登录，然后在控制面板中填写后端地址。",
      deploymentGuide: "完整部署与服务器访问指南",
      protectedAdmin: "管理面板有密码保护",
      persistentRules: "配置持久保存",
      simulationEyebrow: "可选的浏览器模拟说明",
      simulationTitle: "看看一条故障规则如何工作。",
      simulationIntro: "这个小模拟解释“失败两次”规则。要处理你自己应用的真实请求，请使用上方 Docker Compose 或可执行程序运行 FaultDeck。",
      backToSetup: "接入真实后端",
      upstreamResponse: "后端响应",
      interactiveDemo: "交互演示",
      demoTitle: "失败两次，然后恢复。",
      simulation: "模拟演示",
      sampleApi: "示例接口",
      scenarioRule: "前两次请求返回 503",
      requestOne: "第一次请求",
      requestTwo: "第二次请求",
      requestThree: "第三次请求",
      waiting: "等待你发送请求",
      reset: "重置",
      simulationNote: "仅在浏览器中模拟，不会发送真实接口请求。",
      latencyTitle: "让响应变慢",
      latencyDescription: "检查加载状态和客户端超时。",
      errorTitle: "让请求失败",
      errorDescription: "验证限流、错误提示和重试逻辑。",
      disconnectTitle: "断开连接",
      disconnectDescription: "看看收不到响应时会发生什么。",
      dashboardEyebrow: "真实工具界面",
      dashboardTitle: "把意外情况，放进控制面板。",
      dashboardDescription:
        "连接后端，创建故障规则，查看真实请求结果。Docker 部署会在重启后保留目标地址与规则。",
      localDashboard: "自行部署的控制面板",
      screenshotCaption: "FaultDeck 实际界面：注入两次失败，随后请求成功。",
      quickStartEyebrow: "连接你自己的真实应用",
      quickStartTitle: "部署、接入、检查恢复。",
      quickStartDescription:
        "将 FaultDeck 指向你的后端，再让客户端通过 FaultDeck 请求。内置演示仅供选用。",
      stepOneTitle: "启动你的实例",
      stepOneDescription:
        "使用上方 Docker Compose 部署，或下载解压后运行可执行程序：",
      chooseDownload: "选择下载版本",
      stepTwoTitle: "连接你的后端",
      stepTwoDescription:
        "打开控制面板，将目标地址设为真实后端。该地址需要能被 FaultDeck 所在服务器访问。",
      stepTwoAside:
        "初始目标可能是内置演示。请替换为你的服务地址并保存。",
      stepThreeTitle: "发送真实应用请求",
      stepThreeDescription:
        "将客户端 Base URL 改为面板显示的代理地址。需要鉴权时，从部署配置取值并添加 X-FaultDeck-Token 请求头。",
      stepThreeAside:
        "为你的接口创建规则，从应用发送真实请求，查看活动记录。失败两次后，第三次请求会正常转发给后端。",
      closingEyebrow: "为异常路径而生",
      closingTitle: "在用户遇到之前，先发现故障。",
      closingDescription:
        "试一个场景，告诉我们哪里出了问题，一起把开发工具做得更好。",
      viewGitHub: "在 GitHub 查看",
      feedback: "反馈问题或提出想法",
      footerTagline: "小故障，更可靠的软件。",
      license: "MIT 开源许可",
      privacy: "无统计追踪，无 Cookie。",
      sending: "正在模拟…",
      complete: "已恢复 · 3 / 3",
      send: (number) => `发送第 ${number} 次请求`,
      initialProgress: "点击按钮，发送第一次模拟请求。",
      busyProgress: (number) => `正在模拟第 ${number} / 3 次请求…`,
      firstProgress: "已注入 503 故障。再发一次请求，看看接下来会怎样。",
      secondProgress: "已经注入两次故障。下一次请求可以正常通过。",
      finalProgress: "200 OK。规则已达到次数上限，第三次请求成功。",
      failedDetail: "故障已注入 · 服务暂不可用",
      recoveredDetail: "规则次数已用完 · 请求正常通过",
      screenshotAlt:
        "FaultDeck 控制面板：已注入两次 503 故障。规则达到次数上限，下一次请求返回 200。",
      traceLabel: "模拟请求结果",
      statusesLabel: "失败两次规则：503、503，然后返回真实后端响应",
      limit: "上限 2 次",
    },
  };

  let language = "en";
  let completed = 0;
  let pending = false;
  let timer = null;
  const sendButton = document.getElementById("send-request");
  const sendLabel = document.getElementById("send-label");
  const resetButton = document.getElementById("reset-demo");
  const progress = document.getElementById("demo-progress");
  const languageButton = document.getElementById("language-toggle");
  const rows = Array.from(document.querySelectorAll(".trace-row"));

  function renderDemo() {
    const t = words[language];
    rows.forEach((row, index) => {
      const done = index < completed;
      row.classList.toggle("failed", done && index < 2);
      row.classList.toggle("recovered", done && index === 2);
      const status = row.querySelector(".trace-status");
      status.textContent = done ? (index < 2 ? "503" : "200") : "—";
      row.querySelector(".trace-detail").textContent = done
        ? index < 2
          ? t.failedDetail
          : t.recoveredDetail
        : t.waiting;
    });
    sendButton.disabled = pending || completed === 3;
    sendLabel.textContent = pending
      ? t.sending
      : completed === 3
        ? t.complete
        : t.send(completed + 1);
    resetButton.disabled = !pending && completed === 0;
    progress.textContent = pending
      ? t.busyProgress(completed + 1)
      : [t.initialProgress, t.firstProgress, t.secondProgress, t.finalProgress][
          completed
        ];
  }

  function setLanguage(next) {
    language = next;
    const t = words[language];
    document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
    document.querySelectorAll("[data-i18n]").forEach((element) => {
      const translation = t[element.dataset.i18n];
      if (typeof translation === "string") element.textContent = translation;
    });
    languageButton.textContent = language === "zh" ? "EN" : "中文";
    languageButton.setAttribute(
      "aria-label",
      language === "zh" ? "Switch to English" : "切换为中文",
    );
    document.getElementById("dashboard-image").alt = t.screenshotAlt;
    document.querySelectorAll("[data-deploy-doc]").forEach((link) => {
      link.href = language === "zh"
        ? "https://github.com/ogrtdtghkhan-afk/faultdeck/blob/main/docs/deployment.zh-CN.md"
        : "https://github.com/ogrtdtghkhan-afk/faultdeck/blob/main/docs/deployment.md";
    });
    document
      .querySelector(".request-trace")
      .setAttribute("aria-label", t.traceLabel);
    document
      .querySelector(".result-sequence")
      .setAttribute("aria-label", t.statusesLabel);
    document.querySelector(".rule-limit").textContent = t.limit;
    document.title =
      language === "zh"
        ? "FaultDeck — 在这里制造故障，让上线更有底气。"
        : "FaultDeck — Break it here. Ship it stronger.";
    renderDemo();
  }

  sendButton.addEventListener("click", () => {
    if (pending || completed === 3) return;
    const startedWithFocus = document.activeElement === sendButton;
    pending = true;
    renderDemo();
    // Deliberately a browser-only simulation: no fetch, XHR or backend request.
    timer = window.setTimeout(
      () => {
        timer = null;
        pending = false;
        completed += 1;
        rows.forEach((row) => row.classList.remove("just-completed"));
        rows[completed - 1].classList.add("just-completed");
        renderDemo();
        // Preserve keyboard progress without stealing focus if someone tabbed away.
        if (
          startedWithFocus &&
          (document.activeElement === document.body ||
            document.activeElement === sendButton)
        ) {
          (completed === 3 ? resetButton : sendButton).focus({
            preventScroll: true,
          });
        }
      },
      window.matchMedia("(prefers-reduced-motion: reduce)").matches ? 0 : 320,
    );
  });

  resetButton.addEventListener("click", () => {
    if (timer !== null) window.clearTimeout(timer);
    timer = null;
    pending = false;
    completed = 0;
    rows.forEach((row) => row.classList.remove("just-completed"));
    renderDemo();
    sendButton.focus({ preventScroll: true });
  });

  languageButton.addEventListener("click", () =>
    setLanguage(language === "en" ? "zh" : "en"),
  );
  setLanguage("en");
})();
