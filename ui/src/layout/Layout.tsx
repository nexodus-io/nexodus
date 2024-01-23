import { Layout, LayoutProps } from "react-admin";
import AppBar from "./AppBar";
import { FlagsProvider } from "../common/FlagsContext";

export default (props: LayoutProps) => {
  return (
    <FlagsProvider>
      <Layout {...props} appBar={AppBar} />
    </FlagsProvider>
  );
};
