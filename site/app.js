"use strict";

(() => {
  const words = {
    en: {
      skip: "Skip to content",
      docs: "Docs",
      source: "Source",
      eyebrow: "YOUR LOCAL FAILURE LAB",
      heroLineOne: "Break it here.",
      heroLineTwo: "Ship it stronger.",
      heroDescription:
        "Slow responses. Failed requests. Dropped connections. Make them happen on purpose—and see how your app recovers.",
      download: "Download FaultDeck",
      quickStartLink: "Get started",
      heroNote: "Free & open source · Windows, macOS & Linux",
      trustLine: "Runs on your machine. No account. No cloud.",
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
        "Choose a fault, send a request, and see exactly which rule fired. Save the scenario to run it again.",
      localDashboard: "LOCAL DASHBOARD",
      screenshotCaption:
        "Actual FaultDeck dashboard. Two injected failures, followed by a successful response.",
      quickStartEyebrow: "YOUR FIRST FAILURE, IN MINUTES",
      quickStartTitle: "Bring your app. Or try ours.",
      quickStartDescription:
        "A demo API is included. Start testing without an API key, a dataset, or your own backend.",
      stepOneTitle: "Download & start",
      stepOneDescription:
        "Extract the release for your system. Open a terminal in that folder and run:",
      chooseDownload: "Choose your download",
      stepTwoTitle: "Open the control panel",
      stepTwoDescription:
        "Visit this local address in your browser. The built-in demo is already connected.",
      stepTwoAside:
        "Ready for your app? Set your backend as the target and use port 7332 as your app’s API address.",
      stepThreeTitle: "Fail twice. Check recovery.",
      stepThreeDescription:
        "Choose the “Fail twice” preset. Send three requests to /api/orders in the playground.",
      stepThreeAside:
        "Now those are real requests. Check the activity log to see what happened.",
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
      statusesLabel: "Expected demo statuses: 503, 503, 200",
      limit: "LIMIT 2",
    },
    zh: {
      skip: "跳转到正文",
      docs: "文档",
      source: "源码",
      eyebrow: "你的本地故障实验室",
      heroLineOne: "在这里制造故障。",
      heroLineTwo: "让上线更有底气。",
      heroDescription:
        "响应变慢、请求失败、连接中断。主动制造这些情况，看看你的应用能否正确恢复。",
      download: "下载 FaultDeck",
      quickStartLink: "开始使用",
      heroNote: "免费开源 · Windows、macOS 和 Linux",
      trustLine: "在本机运行，无需账号，无需云服务。",
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
        "选择故障、发送请求，清楚看到哪条规则生效。保存场景，下次继续使用。",
      localDashboard: "本地控制面板",
      screenshotCaption: "FaultDeck 实际界面：注入两次失败，随后请求成功。",
      quickStartEyebrow: "几分钟内，触发第一次故障",
      quickStartTitle: "接入你的应用，或先试试内置示例。",
      quickStartDescription:
        "自带演示接口，无需 API Key、数据集，也无需准备自己的后端。",
      stepOneTitle: "下载并启动",
      stepOneDescription:
        "下载适合你系统的版本并解压。在该文件夹打开终端，运行：",
      chooseDownload: "选择下载版本",
      stepTwoTitle: "打开控制面板",
      stepTwoDescription:
        "在浏览器中打开下面的本地地址。内置演示服务已自动连接。",
      stepTwoAside:
        "要测试自己的应用？将后端设为目标地址，再让应用通过 7332 端口请求接口。",
      stepThreeTitle: "失败两次，检查恢复",
      stepThreeDescription:
        "选择“Fail twice”预设。在请求面板中向 /api/orders 发送三次请求。",
      stepThreeAside:
        "现在发送的就是真实请求了。在活动记录中查看每次请求的结果。",
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
      statusesLabel: "演示预期状态码：503、503、200",
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
