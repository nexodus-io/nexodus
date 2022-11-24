import { useState, useEffect } from "react";
import { styled } from "@mui/material/styles";
import { useLogin, Form, Login, LoginFormProps } from "react-admin";
import { CardContent, Button, CircularProgress } from "@mui/material";

const LoginForm = (props: LoginFormProps) => {
  const { redirectTo, className } = props;
  const [loading, setLoading] = useState(false);
  const login = useLogin();

  useEffect(() => {
    const { searchParams } = new URL(window.location.href);
    const code = searchParams.get("code");
    const state = searchParams.get("state");

    // If code is present, we came back from the provider
    if (code && state) {
      console.log("handling return from login");
      setLoading(true);
      login({ code, state });
    }
  }, [login]);

  const handleLogin = () => {
    console.log("login button pressed");
    setLoading(true);
    login({}); // Do not provide code, just trigger the redirection
  };

  return (
    <StyledForm
      onSubmit={handleLogin}
      mode="onChange"
      noValidate
      className={className}
    >
      <CardContent className={LoginFormClasses.content}>
        <Button
          variant="contained"
          type="submit"
          color="primary"
          disabled={loading}
          fullWidth
          className={LoginFormClasses.button}
        >
          {loading && <CircularProgress size={18} thickness={2} />}
          Login
        </Button>
      </CardContent>
    </StyledForm>
  );
};

const PREFIX = "RaLoginForm";

export const LoginFormClasses = {
  content: `${PREFIX}-content`,
  button: `${PREFIX}-button`,
  icon: `${PREFIX}-icon`,
};

const StyledForm = styled(Form, {
  name: PREFIX,
  overridesResolver: (props, styles) => styles.root,
})(({ theme }) => ({
  [`& .${LoginFormClasses.content}`]: {
    width: 300,
  },
  [`& .${LoginFormClasses.button}`]: {
    marginTop: theme.spacing(2),
  },
  [`& .${LoginFormClasses.icon}`]: {
    margin: theme.spacing(0.3),
  },
}));

const LoginPage = () => (
  <Login
    backgroundImage="https://source.unsplash.com/fR47SivxkSM"
    children={<LoginForm />}
  />
);

export default LoginPage;
