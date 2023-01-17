import { Layout, LayoutProps } from "react-admin";
import AppBar from "./AppBar";

export default (props: LayoutProps) => {
  return <Layout {...props} appBar={AppBar} />;
};
