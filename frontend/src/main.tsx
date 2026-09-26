import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles.css";

// React 应用挂载点；具体页面状态和业务交互由 App 统一管理。
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
