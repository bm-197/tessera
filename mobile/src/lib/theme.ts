import { Platform } from "react-native";

export const C = {
  bg: "#1b1e24",
  panel: "#20242c",
  panel2: "#262b34",
  border: "#2c313b",
  text: "#e7e9ee",
  muted: "#8b909c",
  dim: "#5b616e",
  green: "#bcd449",
  purple: "#c4a7f0",
  yellow: "#e8c45c",
  red: "#e06c6c",
  ink: "#1b1e24",
};

export const MONO = Platform.select({
  ios: "Menlo",
  android: "monospace",
  default: "monospace",
}) as string;
