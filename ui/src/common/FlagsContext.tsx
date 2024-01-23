import {
  useState,
  createContext,
  useEffect,
  PropsWithChildren,
  ReactNode,
  useContext,
} from "react";
import { dataProvider } from "../DataProvider";
import { useGetIdentity } from "react-admin";

const FlagsContext = createContext(
  {} as {
    [index: string]: boolean;
  },
);

export function FlagsProvider(props: PropsWithChildren<{}>) {
  const { data, isLoading } = useGetIdentity();
  const [flags, setFlags] = useState({} as { [index: string]: boolean });
  useEffect(() => {
    isLoading ||
      (async () => {
        try {
          const flags = await dataProvider.getFlags();
          setFlags(flags);
        } catch (e) {
          console.log(e);
        }
      })();
  }, [data]); // we can only fetch flags after we have logged in

  return (
    <FlagsContext.Provider value={flags}>
      {props.children}
    </FlagsContext.Provider>
  );
}

export const useFlags = () => {
  return useContext(FlagsContext);
};
