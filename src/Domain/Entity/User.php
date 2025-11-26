<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Entity;

use Symfony\Component\Security\Core\User\PasswordAuthenticatedUserInterface;

readonly class User implements PasswordAuthenticatedUserInterface
{
    public function __construct(
        public string $lastName,
        public string $firstName,
        public string $email,
        public string $password,
    ) {
    }

    public function getPassword(): ?string
    {
        return $this->password;
    }
}
